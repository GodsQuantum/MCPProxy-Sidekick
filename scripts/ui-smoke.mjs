const baseURL = process.argv[2] || "http://127.0.0.1:18081/";
const cdpPort = process.argv[3] || "9222";
const adminKey = process.argv[4] || "admin-key";
const startupDeadline = Date.now() + 20000;
const sleep = ms => new Promise(r => setTimeout(r, ms));

async function tabs(){
  return await (await fetch("http://127.0.0.1:"+cdpPort+"/json/list")).json();
}
let tab;
while(Date.now()<startupDeadline){
  try{ tab=(await tabs()).find(x=>x.type==="page"); if(tab)break; }catch{}
  await sleep(200);
}
if(!tab) throw new Error("Chrome CDP page target unavailable");

const ws=new WebSocket(tab.webSocketDebuggerUrl);
let seq=1;
const pending=new Map();
const call=(method,params={})=>new Promise((resolve,reject)=>{
  const id=seq++;
  const timer=setTimeout(()=>{pending.delete(id);reject(new Error("CDP timeout: "+method))},10000);
  pending.set(id,{resolve,reject,timer});
  ws.send(JSON.stringify({id,method,params}));
});
ws.onmessage=event=>{
  const msg=JSON.parse(event.data);
  const p=pending.get(msg.id); if(!p)return;
  pending.delete(msg.id); clearTimeout(p.timer);
  if(msg.error) p.reject(new Error(JSON.stringify(msg.error))); else p.resolve(msg.result);
};
await new Promise((resolve,reject)=>{ws.onopen=resolve;ws.onerror=reject});

const evaluate=async expression=>{
  const r=await call("Runtime.evaluate",{expression,returnByValue:true,awaitPromise:true});
  if(r.exceptionDetails) throw new Error(JSON.stringify(r.exceptionDetails));
  return r.result.value;
};
const waitFor=async(expression,message)=>{
  const until=Date.now()+12000;
  while(Date.now()<until){
    if(await evaluate(expression)) return;
    await sleep(150);
  }
  throw new Error(message);
};
const assert=(condition,message)=>{if(!condition)throw new Error(message)};

await call("Page.navigate",{url:baseURL});
await waitFor(
  "location.href.startsWith("+JSON.stringify(baseURL)+") && document.readyState==='complete' && !!document.querySelector('#login')",
  "Sidekick page/login did not load",
);
await waitFor("!document.querySelector('#login').hidden","login did not appear");
await evaluate("document.querySelector('#admin-key').value="+JSON.stringify(adminKey)+";document.querySelector('#login-form').requestSubmit();true");
await waitFor("!document.querySelector('#app').hidden && document.querySelector('#profiles-list')?.innerText.includes('/mcp/p/personal')","unlock/state load failed");

assert(await evaluate("document.querySelector('#modal > .modal-card')?.tagName==='DIV'"),"modal-card must not be a FORM");
await evaluate("window.confirm=()=>true;document.querySelector('#new-profile').click();true");
assert(await evaluate("!!document.querySelector('#profile-form')"),"profile form missing");
await evaluate("document.querySelector('#profile-name').value='browser-test';document.querySelector('#profile-form').requestSubmit();true");
await waitFor("document.querySelector('#profiles-list')?.innerText.includes('/mcp/p/browser-test')","created profile not rendered");

await evaluate("document.querySelector('.assign-server[data-profile=\"browser-test\"]').click();true");
assert(await evaluate("!!document.querySelector('#assign-form')"),"assign form missing");
await evaluate("document.querySelector('#assign-server').value='filesystem';document.querySelector('#assign-form').requestSubmit();true");
await waitFor("Array.from(document.querySelectorAll('.profile-card')).some(x=>x.innerText.includes('browser-test')&&x.innerText.includes('filesystem'))","assigned upstream not rendered");

await evaluate("document.querySelector('.remove-profile-server[data-profile=\"browser-test\"][data-server=\"filesystem\"]').click();true");
await waitFor("!document.querySelector('.remove-profile-server[data-profile=\"browser-test\"][data-server=\"filesystem\"]')","upstream removal not rendered");

await evaluate("document.querySelector('#new-token').click();true");
assert(await evaluate("!!document.querySelector('#token-form')"),"token form missing");
await evaluate("document.querySelector('.modal-close').click();openCredential('github');true");
assert(await evaluate("!!document.querySelector('#credential-form')"),"credential form missing");
await evaluate("document.querySelector('.modal-close').click();true");

const before=(await tabs()).filter(x=>x.type==="page").length;
await call("Runtime.evaluate",{expression:"document.querySelector('.oauth-start[data-name=\"google-oauth\"]').click();true",returnByValue:true,userGesture:true});
let popupError=false;
const popupDeadline=Date.now()+12000;
while(Date.now()<popupDeadline){
  const pages=(await tabs()).filter(x=>x.type==="page");
  if(pages.length>before){
    for(const p of pages){
      if(p.id===tab.id)continue;
      const s=new WebSocket(p.webSocketDebuggerUrl);
      await new Promise((resolve,reject)=>{s.onopen=resolve;s.onerror=reject});
      const id=1;
      const result=await new Promise((resolve,reject)=>{
        const timer=setTimeout(()=>reject(new Error("popup CDP timeout")),3000);
        s.onmessage=e=>{const m=JSON.parse(e.data);if(m.id===id){clearTimeout(timer);resolve(m)}};
        s.send(JSON.stringify({id,method:"Runtime.evaluate",params:{expression:"document.body?.innerText||''",returnByValue:true}}));
      }).catch(()=>null);
      s.close();
      if(result?.result?.result?.value?.includes("OAuth start failed")) popupError=true;
    }
    if(popupError)break;
  }
  await sleep(200);
}
assert(popupError,"OAuth failure popup was closed or did not preserve the error");

await evaluate("document.querySelector('.delete-profile[data-profile=\"browser-test\"]').click();true");
await waitFor("!document.querySelector('#profiles-list')?.innerText.includes('/mcp/p/browser-test')","profile deletion not rendered");

console.log("Sidekick browser E2E smoke: PASS");
ws.close();
