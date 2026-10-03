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
assert(await evaluate("!!document.querySelector('#login-status')"),"login status live region missing");
const wrongBusy=await evaluate("(()=>{const i=document.querySelector('#admin-key');i.value='wrong-key';document.querySelector('#login-form').requestSubmit();return document.querySelector('#login-form').getAttribute('aria-busy')==='true'&&document.querySelector('#login-form button[type=submit]').disabled})()");
assert(wrongBusy,"login did not enter checking state for invalid key");
await waitFor("document.querySelector('#login-status')?.textContent.includes('Connection failed')","login error status missing");
assert(await evaluate("document.querySelector('#admin-key').value==='wrong-key' && !document.querySelector('#login-form button[type=submit]').disabled"),"failed login must preserve key and unlock submit");
const goodBusy=await evaluate("(()=>{const i=document.querySelector('#admin-key');i.value="+JSON.stringify(adminKey)+";document.querySelector('#login-form').requestSubmit();return document.querySelector('#login-form').getAttribute('aria-busy')==='true'&&document.querySelector('#login-form button[type=submit]').disabled})()");
assert(goodBusy,"login did not enter checking state");
await waitFor("document.querySelector('#login-status')?.textContent.includes('Connected to MCPProxy')","login success status missing");
await waitFor("!document.querySelector('#app').hidden && document.querySelector('#profiles-list')?.innerText.includes('/mcp/p/personal')","unlock/state load failed");
await waitFor("document.querySelector('#refresh-state')?.dataset.mode==='sse' && Number(document.querySelector('#refresh-state')?.dataset.fallbacks||0)>=1","SSE fallback/reconnect cycle was not observed");
assert(await evaluate("!document.body.innerText.includes('SUPER_SECRET_SSE_SMOKE')"),"raw MCPProxy SSE payload leaked into the UI");
await evaluate("document.querySelector('.nav-item[data-view=\"activity\"]').click();true");
await waitFor("document.querySelector('#activity-list')?.innerText.includes('servers.changed')","normalized SSE activity did not render");
const navText=await evaluate("Array.from(document.querySelectorAll('#nav .nav-item')).map(x=>x.textContent.trim()).join('|')");
assert(navText==="Overview|Connections|Profiles|Agents|Activity & Security|Settings","unexpected V2 navigation: "+navText);
await evaluate("document.querySelector('.nav-item[data-view=\"connections\"]').click();true");
await waitFor("document.querySelector('#connections-list .connection-card')?.innerText.includes('github')","Connections view did not render");
assert(await evaluate("!!document.querySelector('.connection-open[data-name=\"google-oauth\"]')"),"connection detail action missing");
await evaluate("document.querySelector('.connection-open[data-name=\"google-oauth\"]').click();true");
await waitFor("document.querySelector('#modal[open] details summary')?.textContent.includes('Advanced')","connection Advanced disclosure missing");
await evaluate("document.querySelector('.modal-close').click();true");
await waitFor("!document.querySelector('#modal').open","connection dialog did not close");
assert(await evaluate("document.activeElement?.classList.contains('connection-open') && document.activeElement?.dataset.name==='google-oauth'"),"dialog focus did not return to connection opener");
await evaluate("document.querySelector('.nav-item[data-view=\"settings\"]').click();true");
await waitFor("document.querySelector('#browser-provider-card')?.innerText.includes('Chromium')","browser provider Settings card missing");
assert(await evaluate("document.querySelector('#browser-provider-card')?.innerText.includes('Bitwarden: off')"),"Bitwarden mode missing");
await evaluate("document.querySelector('.nav-item[data-view=\"profiles\"]').click();true");
await waitFor("!!document.querySelector('.profile-advanced[data-profile=\"personal\"]')","Profile v3 policy action missing when capability is present");
await evaluate("document.querySelector('.profile-advanced[data-profile=\"personal\"]').click();true");
await waitFor("!!document.querySelector('#profile-advanced-form')","Profile v3 editor did not open");
assert(await evaluate("!!document.querySelector('#profile-max-tier') && !!document.querySelector('#profile-unannotated') && !!document.querySelector('#profile-classify')"),"Profile v3 policy controls missing");
await evaluate("document.querySelector('#profile-try-query').value='issue';document.querySelector('#profile-try-btn').click();true");
await waitFor("document.querySelector('#profile-try-result')?.textContent.includes('Hidden by profile: 1')","Profile v3 Try policy result missing");
await evaluate("document.querySelector('.modal-close').click();true");
await waitFor("!document.querySelector('#modal').open","Profile policy dialog did not close");

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

await evaluate("document.querySelector('.nav-item[data-view=\"agents\"]').click();document.querySelector('#new-token').click();true");
assert(await evaluate("!!document.querySelector('#token-form')"),"agent onboarding form missing");
assert(await evaluate("!!document.querySelector('#token-profile[required]') && !Array.from(document.querySelectorAll('#token-profile option')).some(x=>x.value==='')"),"agent onboarding must require a profile");
assert(await evaluate("!!document.querySelector('#agent-target option[value=\"n8n\"]')"),"agent target selector missing n8n");
await evaluate("document.querySelector('#token-name').value='smoke-agent';document.querySelector('#token-profile').value='personal';document.querySelector('#agent-target').value='n8n';document.querySelector('#token-form').requestSubmit();true");
await waitFor("document.querySelector('#one-token')?.value==='mcp_agt_smoke_once'","one-time agent token missing");
assert(await evaluate("document.querySelector('#one-time-snippet')?.value.includes('/mcp/p/personal') && document.querySelector('#one-time-snippet')?.value.includes('Authorization')"),"n8n onboarding snippet missing");
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
