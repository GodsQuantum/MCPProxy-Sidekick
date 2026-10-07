"use strict";

const state={data:null,connections:[],csrf:"",view:"overview",mcpproxyVersion:"",modalReturnFocus:null,activityEvents:[],eventSource:null,eventFallbacks:0,eventRetryStreak:0,eventReconnectTimer:null,pollTimer:null,refreshTimer:null};
const configuredBase=document.querySelector('meta[name="sidekick-mount-path"]')?.content?.trim()||"";
const BASE=(configuredBase||location.pathname).replace(/\/+$/,"");
const $=(s,r=document)=>r.querySelector(s);
const $$=(s,r=document)=>Array.from(r.querySelectorAll(s));
const esc=v=>String(v??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
const titleCase=v=>String(v||"").replace(/(^|[-_ ])([a-z])/g,(_,a,b)=>a+b.toUpperCase());

function toast(message){
  const e=$("#toast"); e.textContent=message; e.hidden=false;
  clearTimeout(toast.timer); toast.timer=setTimeout(()=>{e.hidden=true},4200);
}

function setLoginState(kind,message){
  const form=$("#login-form"),button=form?.querySelector('button[type="submit"]'),label=button?.querySelector(".button-label"),status=$("#login-status");
  const checking=kind==="checking";
  if(form) form.setAttribute("aria-busy",checking?"true":"false");
  if(button) button.disabled=checking;
  if(label) label.textContent=checking?"Checking MCPProxy…":(kind==="success"?"Connected":"Unlock Sidekick");
  if(status){
    status.className="login-status "+(kind==="success"?"good":kind==="error"?"bad":"muted");
    status.textContent=message||"";
  }
}

async function api(path,opt={}){
  opt.headers={Accept:"application/json",...(opt.headers||{})};
  if(opt.body){
    opt.headers["Content-Type"]="application/json";
    if(state.csrf) opt.headers["X-CSRF-Token"]=state.csrf;
  }
  const r=await fetch(BASE+path,opt);
  const text=await r.text();
  let data={}; try{data=text?JSON.parse(text):{}}catch{}
  if(!r.ok){
    const err=new Error(data.error||("HTTP "+r.status));
    err.status=r.status;
    throw err;
  }
  return data;
}

function setLiveMode(mode,label){
  const el=$("#refresh-state");
  if(!el)return;
  el.dataset.mode=mode;
  el.dataset.fallbacks=String(state.eventFallbacks);
  el.textContent=label||(mode==="sse"?"Live":mode==="polling"?"Polling":"Offline");
}

function stopPollingFallback(){
  if(state.pollTimer){clearInterval(state.pollTimer);state.pollTimer=null}
}

function startPollingFallback(){
  if(state.pollTimer)return;
  state.pollTimer=setInterval(()=>{
    if(!document.hidden&&!$("#app").hidden)load({allowEventStart:false});
  },30000);
}

function stopLiveUpdates(){
  if(state.eventSource){state.eventSource.close();state.eventSource=null}
  if(state.eventReconnectTimer){clearTimeout(state.eventReconnectTimer);state.eventReconnectTimer=null}
  if(state.refreshTimer){clearTimeout(state.refreshTimer);state.refreshTimer=null}
  stopPollingFallback();
}

function recordActivityEvent(ev){
  if(!ev?.event||ev.kind==="ready")return;
  state.activityEvents.unshift({event:String(ev.event),kind:String(ev.kind||"state"),at:new Date().toLocaleTimeString()});
  state.activityEvents=state.activityEvents.slice(0,12);
  renderAttention();
}

async function refreshConnectionsOnly(){
  try{
    const c=await api("/api/connections");
    state.connections=c.connections||[];
    if(state.data){
      const list=state.connections;
      state.data.summary={
        ...(state.data.summary||{}),
        total:list.length,
        connected:list.filter(x=>x.ready).length,
        tools:list.reduce((n,x)=>n+(Number(x.tool_count)||0),0),
        auth_required:list.filter(x=>(x.oauth&&!x.authenticated)||/auth/i.test(String(x.status||""))).length,
        quarantined:list.filter(x=>x.quarantined).length,
      };
      renderSummary();renderConnections();renderAttention();
    }
  }catch{ scheduleStateRefresh(150) }
}

function scheduleStateRefresh(delay=150){
  clearTimeout(state.refreshTimer);
  state.refreshTimer=setTimeout(()=>{state.refreshTimer=null;load({allowEventStart:false})},delay);
}

function scheduleEventReconnect(){
  if(state.eventReconnectTimer||document.hidden||$("#app").hidden)return;
  state.eventRetryStreak++;
  const exponent=Math.min(state.eventRetryStreak-1,5);
  const base=Math.min(15000,750*(2**exponent));
  const jitter=Math.floor(Math.random()*250);
  state.eventReconnectTimer=setTimeout(()=>{
    state.eventReconnectTimer=null;
    if(!document.hidden&&!$("#app").hidden)startLiveUpdates();
  },base+jitter);
}

function enterEventFallback(ev){
  if(ev)recordActivityEvent(ev);
  state.eventFallbacks++;
  setLiveMode("polling","Polling");
  startPollingFallback();
  if(state.eventSource){
    const es=state.eventSource;
    state.eventSource=null;
    es.close();
  }
  scheduleEventReconnect();
}

function handleLiveEvent(ev){
  if(!ev||typeof ev!=="object")return;
  if(ev.kind==="ready"){setLiveMode("sse",(state.data?.warnings||[]).length?"Degraded":"Live");return}
  if(ev.kind==="fallback"){enterEventFallback(ev);return}
  recordActivityEvent(ev);
  if(ev.kind==="connections")refreshConnectionsOnly();
  else if(ev.kind==="activity")renderAttention();
  else scheduleStateRefresh(120);
}

function startLiveUpdates(){
  if($("#app").hidden||document.hidden)return;
  if(typeof EventSource==="undefined"){
    setLiveMode("polling","Polling");
    startPollingFallback();
    return;
  }
  if(state.eventSource&&state.eventSource.readyState!==EventSource.CLOSED)return;
  if(state.eventReconnectTimer)return;
  const es=new EventSource(BASE+"/api/events");
  state.eventSource=es;
  es.onopen=()=>{
    if(state.eventSource!==es)return;
    state.eventRetryStreak=0;
    stopPollingFallback();
    setLiveMode("sse",(state.data?.warnings||[]).length?"Degraded":"Live");
  };
  es.onmessage=e=>{
    try{handleLiveEvent(JSON.parse(e.data))}catch{}
  };
  es.onerror=()=>{
    if(state.eventSource!==es)return;
    enterEventFallback({kind:"fallback",event:"sidekick.events.disconnected"});
  };
}

function badge(status,enabled=true,quarantined=false){
  if(quarantined) return '<span class="badge bad">Quarantined</span>';
  if(!enabled) return '<span class="badge warn">Disabled</span>';
  const s=String(status||"unknown").toLowerCase();
  const cls=/ready|connected/.test(s)?"good":(/auth|pending|disabled/.test(s)?"warn":(/error|failed|unhealthy/.test(s)?"bad":""));
  return '<span class="badge '+cls+'">'+esc(status||"unknown")+'</span>';
}

async function load({allowEventStart=true}={}){
  $("#refresh-state").textContent="Refreshing…";
  try{
    const d=await api("/api/state");
    state.data=d; state.csrf=d.csrf||state.csrf;
    try{
      const c=await api("/api/connections");
      state.connections=c.connections||[];
    }catch{
      state.connections=(d.upstreams||[]).map(x=>({...x,ready:/ready|connected/i.test(x.status||""),actions:[x.enabled?"disable":"enable","restart",...(x.oauth?["reconnect"]:[])]}));
    }
    $("#login").hidden=true; $("#app").hidden=false;
    render(); $("#refresh-state").textContent=(d.warnings||[]).length?"Degraded":"Live";
    if((d.warnings||[]).length) toast(d.warnings.join(" · "));
    if(allowEventStart)startLiveUpdates();
  }catch(e){
    if(e?.status===401||/401|authentication|session expired/i.test(e.message)){
      stopLiveUpdates();
      $("#app").hidden=true; $("#login").hidden=false;
      setLoginState("idle","Enter your MCPProxy admin key.");
    } else toast(e.message);
    $("#refresh-state").textContent="Offline";
  }
}

function render(){
  renderSummary();
  renderProfiles();
  renderConnections();
  renderTokens();
  renderAttention();
  renderSettings();
}

function renderSummary(){
  const s=state.data?.summary||{};
  const items=[["Connections",s.total||0],["Ready",s.connected||0],["Tools",s.tools||0],["Auth needed",s.auth_required||0],["Quarantined",s.quarantined||0]];
  $("#summary").innerHTML=items.map(x=>'<article class="stat"><strong>'+esc(x[1])+'</strong><span>'+esc(x[0])+'</span></article>').join("");
}

function renderProfiles(){
  const ps=state.data?.profiles||[];
  const html=ps.length?ps.map(p=>{
    const servers=p.servers||[];
    const chips=servers.map(n=>'<span class="chip">'+esc(n)+' <button class="chip-x remove-profile-server" type="button" data-profile="'+esc(p.name)+'" data-server="'+esc(n)+'" aria-label="Remove '+esc(n)+'">×</button></span>').join("");
    const advanced=state.data?.mcpproxy?.profile_v3?'<button class="secondary profile-advanced" data-profile="'+esc(p.name)+'">Policy</button>':'';
    return '<article class="profile-card"><div class="row"><div><h3>'+esc(p.name)+'</h3><span class="muted">/mcp/p/'+esc(p.name)+'</span></div><span class="badge">'+esc(p.tool_count||0)+' tools</span></div><div class="chips">'+(chips||'<span class="muted">Deny-all until a connection is assigned</span>')+'</div><div class="card-actions">'+advanced+'<button class="secondary assign-server" data-profile="'+esc(p.name)+'">Add connection</button><button class="ghost delete-profile" data-profile="'+esc(p.name)+'">Delete</button></div></article>';
  }).join(""):'<div class="empty">No MCPProxy profiles yet.</div>';
  $("#profiles-list").innerHTML=html; $("#overview-profiles").innerHTML=html;
  $$(".profile-advanced").forEach(b=>b.onclick=()=>openProfileAdvanced(b.dataset.profile,b));
  $$(".assign-server").forEach(b=>b.onclick=()=>openAssign(b.dataset.profile,b));
  $$(".remove-profile-server").forEach(b=>b.onclick=()=>removeProfileServer(b.dataset.profile,b.dataset.server));
  $$(".delete-profile").forEach(b=>b.onclick=()=>deleteProfile(b.dataset.profile));
}

function connectionCard(c){
  const memberships=(c.profiles||[]).join(", ")||"Unassigned";
  const auth=c.oauth?"OAuth":(c.credential_configured?"API credential":"No auth detected");
  let quick=c.oauth
    ?'<button class="secondary oauth-start" data-name="'+esc(c.name)+'">'+(c.authenticated?"Reconnect OAuth":"Connect OAuth")+'</button>'
    :'<button class="secondary credential-open" data-name="'+esc(c.name)+'">Set credential</button>';
  return '<article class="server-card connection-card" data-name="'+esc(c.name)+'">'+
    '<div class="server-top"><div><h3>'+esc(c.name)+'</h3><span class="muted">'+esc(memberships)+'</span></div>'+badge(c.status,c.enabled,c.quarantined)+'</div>'+
    '<div class="server-meta"><span>'+esc(auth)+' · '+esc(c.protocol||"MCP")+'</span><strong>'+esc(c.tool_count||0)+' tools</strong></div>'+
    '<div class="preview">'+esc(c.credential_preview||(c.oauth?(c.authenticated?"OAuth connected":"OAuth not connected"):"Credential not configured"))+'</div>'+
    '<div class="card-actions"><button class="primary connection-open" data-name="'+esc(c.name)+'">Open</button>'+quick+'</div></article>';
}

function renderConnections(){
  const root=$("#connections-list"); if(!root)return;
  const q=($("#connection-search")?.value||"").toLowerCase();
  const list=(state.connections||[]).filter(c=>!q||c.name.toLowerCase().includes(q)||String(c.status).toLowerCase().includes(q)||(c.profiles||[]).join(" ").toLowerCase().includes(q));
  root.innerHTML=list.length?list.map(connectionCard).join(""):'<div class="empty">No matching connections.</div>';
  $$(".connection-open",root).forEach(b=>b.onclick=()=>openConnection(b.dataset.name,b));
  $$(".oauth-start",root).forEach(b=>b.onclick=()=>startOAuth(b.dataset.name));
  $$(".credential-open",root).forEach(b=>b.onclick=()=>openCredential(b.dataset.name,b));
}

async function openConnection(name,opener){
  try{
    const c=await api("/api/connections/"+encodeURIComponent(name));
    const profiles=(c.profiles||[]).map(x=>'<span class="chip">'+esc(x)+'</span>').join("")||'<span class="muted">Unassigned</span>';
    const scopes=(c.oauth_scopes||[]).join("\n");
    const actions=(c.actions||[]).filter(x=>x!=="reconnect").map(a=>'<button type="button" class="secondary connection-action" data-action="'+esc(a)+'">'+esc(titleCase(a))+'</button>').join("");
    const oauth=c.oauth?'<section class="connection-section"><h3>OAuth scopes</h3><label for="scope-input">Configured scopes<textarea id="scope-input" rows="6">'+esc(scopes)+'</textarea></label><div id="scope-preview" class="muted" role="status" aria-live="polite">One scope per line.</div><div class="card-actions"><button type="button" class="secondary" id="scope-preview-btn">Preview changes</button><button type="button" class="primary" id="scope-apply-btn">Apply scopes</button><button type="button" class="secondary oauth-start" data-name="'+esc(c.name)+'">Reconnect OAuth</button></div></section>':"";
    openModal(
      '<h2 id="modal-title">'+esc(c.name)+'</h2>'+
      '<div class="connection-hero"><div>'+badge(c.status,c.enabled,c.quarantined)+'</div><strong>'+esc(c.tool_count||0)+' tools</strong></div>'+
      '<dl class="detail-grid"><div><dt>Authentication</dt><dd>'+esc(c.oauth?"OAuth":(c.credential_configured?"Credential":"Not configured"))+'</dd></div><div><dt>Profiles</dt><dd class="chips">'+profiles+'</dd></div></dl>'+
      oauth+
      '<div class="card-actions">'+actions+(c.oauth?"":'<button type="button" class="secondary credential-open" data-name="'+esc(c.name)+'">Replace credential</button>')+'</div>'+
      '<details><summary>Advanced</summary><dl class="detail-grid"><div><dt>Protocol</dt><dd>'+esc(c.protocol||"MCP")+'</dd></div><div><dt>Endpoint</dt><dd class="mono">'+esc(c.url||"Managed by MCPProxy")+'</dd></div></dl></details>',
      opener
    );
    $$(".oauth-start",$("#modal")).forEach(b=>b.onclick=()=>startOAuth(b.dataset.name));
    $$(".credential-open",$("#modal")).forEach(b=>b.onclick=()=>openCredential(b.dataset.name,b));
    $$(".connection-action",$("#modal")).forEach(b=>b.onclick=()=>connectionAction(c.name,b.dataset.action));
    if(c.oauth){
      $("#scope-preview-btn").onclick=()=>previewScopes(c.name);
      $("#scope-apply-btn").onclick=()=>applyScopes(c.name);
    }
  }catch(e){toast(e.message)}
}

function scopeInput(){
  return ($("#scope-input")?.value||"").split(/\n|,/).map(x=>x.trim()).filter(Boolean);
}

async function previewScopes(name){
  const out=$("#scope-preview");
  try{
    const d=await api("/api/connections/"+encodeURIComponent(name)+"/oauth-scopes/preview",{method:"POST",body:JSON.stringify({scopes:scopeInput()})});
    const changes=[...(d.added||[]).map(x=>"+"+x),...(d.removed||[]).map(x=>"−"+x)];
    out.textContent=changes.length?changes.join(" · ")+" · Reconnect required": "No scope changes.";
    out.className="scope-status "+(d.reauth_required?"warn-text":"good-text");
  }catch(e){out.textContent=e.message;out.className="scope-status bad-text"}
}

async function applyScopes(name){
  const out=$("#scope-preview");
  try{
    const d=await api("/api/connections/"+encodeURIComponent(name)+"/oauth-scopes/apply",{method:"POST",body:JSON.stringify({scopes:scopeInput()})});
    out.textContent=d.reauth_required?"Scopes applied. Reconnect OAuth to grant the new scope set.":"Scopes unchanged.";
    out.className="scope-status "+(d.reauth_required?"warn-text":"good-text");
    await load();
  }catch(e){out.textContent=e.message;out.className="scope-status bad-text"}
}

async function connectionAction(name,action){
  try{
    await api("/api/connections/"+encodeURIComponent(name),{method:"PATCH",body:JSON.stringify({action})});
    closeModal(); toast(titleCase(action)+" requested for "+name); await load();
  }catch(e){toast(e.message)}
}

function renderAttention(){
  const warnings=state.data?.warnings||[];
  const list=(state.connections||[]).filter(s=>!s.enabled||s.quarantined||!s.ready);
  const warningHtml=warnings.map(w=>'<div class="token-row"><div class="token-info"><h3>Sidekick dependency</h3><p>'+esc(w)+'</p></div><span class="badge warn">Degraded</span></div>').join("");
  const connectionHtml=list.map(s=>'<div class="token-row"><div class="token-info"><h3>'+esc(s.name)+'</h3><p>'+esc(s.status||"unknown")+' · '+(s.tool_count||0)+' tools</p></div>'+badge(s.status,s.enabled,s.quarantined)+'</div>').join("");
  const attentionHtml=warningHtml+connectionHtml||'<div class="empty">Everything looks healthy.</div>';
  if($("#attention"))$("#attention").innerHTML=attentionHtml;

  const eventHtml=state.activityEvents.map(ev=>
    '<div class="token-row"><div class="token-info"><h3>'+esc(ev.event)+'</h3><p>'+esc(titleCase(ev.kind))+' · '+esc(ev.at)+'</p></div><span class="badge">'+esc(ev.kind)+'</span></div>'
  ).join("");
  if($("#activity-list"))$("#activity-list").innerHTML=
    (eventHtml?'<div class="stack">'+eventHtml+'</div>':'<div class="empty">No recent MCPProxy activity.</div>')+
    '<div class="section-head activity-subhead"><div><h3>Needs attention</h3></div></div>'+attentionHtml;
}

function renderSettings(){
  const root=$("#browser-provider-card"); if(!root)return;
  const s=state.data?.settings||{};
  const provider=(s.browser_provider||"chromium").toLowerCase();
  const other=provider==="brave"?"chromium":"brave";
  const instances=Array.isArray(s.browser_instances)?s.browser_instances:[];
  const selected=s.browser_instance||instances[0]?.id||"";
  const options=instances.map(i=>'<option value="'+esc(i.id)+'"'+(i.id===selected?' selected':'')+'>'+esc(i.label||i.id)+'</option>').join("");
  const picker=instances.length>1
    ? '<label>Browser instance<select id="browser-instance-select">'+options+'</select></label>'
    : '<p>Instance: <strong>'+esc(instances[0]?.label||selected||"default")+'</strong></p>';
  root.innerHTML='<div class="row"><div><p class="eyebrow">HUMAN AUTH BROWSER</p><h3>'+esc(titleCase(provider))+'</h3></div><span class="badge good">Configured</span></div>'+
    picker+
    '<div class="card-actions"><button type="button" class="primary" id="open-human-browser"'+(s.browser_open_supported?'':' disabled')+'>Open browser</button></div>'+
    '<p>Bitwarden: <strong>'+esc(s.bitwarden_mode||"off")+'</strong>'+(s.bitwarden_base_url_configured?' · custom vault URL configured':'')+'</p>'+
    '<p class="muted">Only server-declared browser instances can be selected. CDP endpoints are never exposed to the frontend.</p>'+
    '<details><summary>Switch browser engine safely</summary><pre><code>./scripts/configure-browser.sh '+esc(other)+'</code></pre><p class="muted">The script health-checks GUI + CDP and rolls back automatically on failure.</p></details>';
  const open=$("#open-human-browser");
  if(open)open.onclick=()=>window.open(BASE+"/api/browser/open","sidekick-human-browser");
  const select=$("#browser-instance-select");
  if(select)select.onchange=async()=>{
    const previous=selected;
    try{
      await api("/api/settings/browser-instance",{method:"POST",body:JSON.stringify({id:select.value})});
      toast("Browser instance switched to "+select.options[select.selectedIndex].text);
      await load();
    }catch(e){
      select.value=previous;
      toast(e.message);
    }
  };
}

async function restoreOmniRouteMaster(){
  try{
    await api("/api/adapters/omniroute/restore-master",{method:"POST",body:"{}"});
    toast("OmniRoute Master assigned to MCPProxy"); await load();
  }catch(e){toast(e.message)}
}

async function startOAuth(name){
  const popup=window.open("about:blank","mcp-oauth-"+name);
  if(!popup){ toast("Popup blocked. Allow popups for this site and retry."); return; }
  try{ popup.document.title="Starting OAuth"; popup.document.body.textContent="Preparing a fresh OAuth session…"; }catch{}
  try{
    const result=await api("/api/upstreams/"+encodeURIComponent(name)+"/oauth/start",{method:"POST",body:"{}"});
    try{popup.document.body.textContent="Secure browser ready. Opening sign-in…"}catch{}
    popup.location=result.browser_url; popup.focus(); toast("OAuth browser opened for "+name+". Complete sign-in there."); setTimeout(load,2500);
  }catch(e){
    try{popup.document.body.textContent="OAuth start failed: "+e.message}catch{}
    toast("OAuth start failed: "+e.message);
  }
}

function openCredential(name,opener){
  const isOpenAlex=name==="openalex-github";
  const isCryptoLive=name==="cryptocom-live";
  if(name==="cryptocom-market"||name==="cryptocom-paper"){
    toast(name==="cryptocom-market"?"Crypto.com Market Data is public and needs no private credential.":"Crypto.com Paper is local simulation and needs no Crypto.com credential.");
    return;
  }
  const authFields=isOpenAlex
    ? '<p class="muted">OpenAlex GitHub is an optional local fallback. Paste the OpenAlex API key here; Sidekick stores it in a local 0600 secret file. Saving the key does not enable this connection.</p><label>OpenAlex API key<input id="cred-value" type="password" autocomplete="off" required></label>'
    : isCryptoLive
      ? '<p class="muted">Crypto.com Exchange private API requires BOTH the API key and API secret. Sidekick writes the pair to a 0600 transient file, streams it through the existing SSH MCP to the live gateway, restarts only that gateway, then removes the transient files. The secret is never stored in Sidekick metadata.</p><label>Crypto.com API key<input id="cred-value" type="password" autocomplete="off" required></label><label>Crypto.com API secret<input id="cred-secret" type="password" autocomplete="off" required></label><label>Credential lifetime (days)<input id="cred-expiry-days" type="number" min="1" max="365" value="90" required></label>'
      : '<p class="muted">The full secret is submitted once and is not returned by Sidekick afterward.</p><label>Authentication format<select id="cred-mode"><option value="bearer">Authorization: Bearer</option><option value="x-api-key">X-API-Key</option><option value="custom-header">Custom header</option></select></label><label id="header-label" hidden>Header name<input id="cred-header" placeholder="X-Custom-Key"></label><label>API key / token<input id="cred-value" type="password" autocomplete="off" required></label>';
  const submitLabel=isOpenAlex?"Save / replace API key (keep disabled)":isCryptoLive?"SET credentials":"Save / replace & enable";
  openModal('<h2 id="modal-title">Credential · '+esc(name)+'</h2><form id="credential-form" class="form-grid">'+authFields+'<button class="primary" type="submit">'+submitLabel+'</button></form>',opener);
  if(!isOpenAlex&&!isCryptoLive)$("#cred-mode").onchange=e=>{$("#header-label").hidden=e.target.value!=="custom-header"};
  $("#credential-form").onsubmit=async e=>{
    e.preventDefault();
    try{
      const payload=isOpenAlex
        ? {mode:"openalex-api-key",header_name:"",value:$("#cred-value").value}
        : isCryptoLive
          ? {mode:"cryptocom-api-pair",value:$("#cred-value").value,secret:$("#cred-secret").value,expires_days:Number($("#cred-expiry-days").value||90)}
          : {mode:$("#cred-mode").value,header_name:$("#cred-header").value,value:$("#cred-value").value};
      await api("/api/upstreams/"+encodeURIComponent(name)+"/credential",{method:"POST",body:JSON.stringify(payload)});
      closeModal(); toast(name+" credential updated"); await load();
    }catch(err){toast(err.message)}
  };
}

function renderTokens(){
  const list=state.data?.tokens||[];
  $("#tokens-list").innerHTML=list.length?list.map(t=>'<article class="token-row"><div class="token-info"><h3>'+esc(t.name)+'</h3><p>'+esc(t.token_prefix||"")+' · '+esc((t.permissions||[]).join(", "))+' · '+esc((t.allowed_servers||[]).join(", "))+(t.profile_pin?' · profile: '+esc(t.profile_pin):"")+(t.revoked?" · revoked":"")+'</p></div><div class="card-actions"><button class="secondary token-regen" data-name="'+esc(t.name)+'">Regenerate</button><button class="secondary token-revoke" data-name="'+esc(t.name)+'">Revoke</button><button class="ghost token-delete" data-name="'+esc(t.name)+'">Delete</button></div></article>').join(""):'<div class="empty">No Agent Tokens yet.</div>';
  $$(".token-regen").forEach(b=>b.onclick=()=>regenerateToken(b.dataset.name));
  $$(".token-revoke").forEach(b=>b.onclick=()=>tokenDelete(b.dataset.name,false));
  $$(".token-delete").forEach(b=>b.onclick=()=>tokenDelete(b.dataset.name,true));
}

async function regenerateToken(name){
  try{const r=await api("/api/tokens/"+encodeURIComponent(name)+"/regenerate",{method:"POST",body:"{}"});showOneTimeToken(name,r.token);await load()}catch(e){toast(e.message)}
}
async function tokenDelete(name,permanent){
  if(!confirm((permanent?"Permanently delete ":"Revoke ")+name+"?"))return;
  try{await api("/api/tokens/"+encodeURIComponent(name)+(permanent?"/permanent":""),{method:"DELETE",body:"{}"});toast(permanent?"Token deleted":"Token revoked");await load()}catch(e){toast(e.message)}
}
function showOneTimeToken(name,token,snippet=""){
  const snippetHTML=snippet?'<label>Connection snippet<textarea id="one-time-snippet" readonly rows="5" class="mono">'+esc(snippet)+'</textarea></label>':"";
  openModal('<h2 id="modal-title">Save this credential now</h2><p class="muted">MCPProxy shows the secret only once. Closing this dialog removes the plaintext token from Sidekick.</p><label>'+esc(name)+'<input id="one-token" readonly value="'+esc(token)+'"></label>'+snippetHTML+'<div class="card-actions"><button id="copy-token" class="primary">Copy token</button>'+(snippet?'<button id="copy-snippet" class="secondary">Copy connection snippet</button>':"")+'</div>');
  $("#copy-token").onclick=async()=>{await navigator.clipboard.writeText($("#one-token").value);toast("Token copied")};
  if(snippet)$("#copy-snippet").onclick=async()=>{await navigator.clipboard.writeText($("#one-time-snippet").value);toast("Connection snippet copied")};
}

function openToken(){
  const profiles=state.data?.profiles||[];
  if(!profiles.length){toast("Create a profile before giving MCP access to an agent.");return}
  const profileOpts=profiles.map(p=>'<option value="'+esc(p.name)+'">'+esc(p.name)+'</option>').join("");
  openModal('<h2 id="modal-title">Give to an agent</h2><p class="muted">Sidekick creates a credential pinned to one MCPProxy profile. The agent never receives your MCPProxy admin key. If this deployment defines a Dify binding for the selected profile, Sidekick applies it server-side and enforces the configured permissions.</p><form id="token-form" class="form-grid"><label>Agent name<input id="token-name" pattern="[A-Za-z0-9][A-Za-z0-9_-]*" required placeholder="agent-name"></label><label>Profile<select id="token-profile" required>'+profileOpts+'</select></label><label>Client format<select id="agent-target"><option value="generic">Generic MCP HTTP</option><option value="n8n">n8n</option><option value="dify">Dify</option><option value="claude">Claude-compatible</option><option value="codex">Codex-compatible</option></select></label><fieldset><legend>Permissions</legend><div class="chips"><label class="chip"><input type="checkbox" name="perm" value="read" checked> read</label><label class="chip"><input type="checkbox" name="perm" value="write"> write</label><label class="chip"><input type="checkbox" name="perm" value="destructive"> destructive</label></div></fieldset><label>Expiry<input id="token-expiry" value="30d" placeholder="30d"></label><button class="primary" type="submit">Create profile-pinned credential</button></form>');
  $("#token-form").onsubmit=async e=>{
    e.preventDefault();
    const perms=$$('input[name="perm"]:checked').map(x=>x.value);
    const destructive=perms.includes("destructive");
    const confirmation=!destructive||confirm("This credential can call destructive tools. Grant destructive permission?");
    if(!confirmation)return;
    try{
      const r=await api("/api/agents/onboard",{method:"POST",body:JSON.stringify({name:$("#token-name").value,profile:$("#token-profile").value,permissions:perms,expires_in:$("#token-expiry").value,target:$("#agent-target").value,confirm_destructive:confirmation})});
      if(r.applied){closeModal();toast(r.name+" pinned and applied to Dify");await load();return}
      showOneTimeToken(r.name,r.token,r.snippet||"");await load();
    }catch(err){toast(err.message)}
  };
}

function profileSelectOptions(values,current){
  return values.map(v=>'<option value="'+esc(v)+'"'+(v===current?' selected':'')+'>'+esc(v||'Inherit')+'</option>').join("");
}

async function openProfileAdvanced(name,opener){
  try{
    const p=await api("/api/profiles/"+encodeURIComponent(name)+"/advanced");
    const tools=p.tools||{};
    const classify=tools.classify||{};
    const switchable=Array.isArray(p.switchable_to)?p.switchable_to.join("\n"):"";
    const codeValue=p.code_execution===true?"true":p.code_execution===false?"false":"";
    const managementValue=p.management_tools===true?"true":p.management_tools===false?"false":"";
    openModal(
      '<h2 id="modal-title">Profile policy · '+esc(name)+'</h2>'+
      '<p class="muted">These controls are enforced by MCPProxy Profiles v3. Empty policy fields inherit the MCPProxy defaults.</p>'+
      '<form id="profile-advanced-form" class="form-grid">'+
      '<label>Title<input id="profile-title" maxlength="80" value="'+esc(p.title||"")+'"></label>'+
      '<label>Description<textarea id="profile-description" maxlength="500" rows="3">'+esc(p.description||"")+'</textarea></label>'+
      '<div class="detail-grid"><label>Maximum tool tier<select id="profile-max-tier">'+profileSelectOptions(["","read","write","destructive"],p.max_tier||"")+'</select></label>'+
      '<label>Unannotated tools<select id="profile-unannotated">'+profileSelectOptions(["","deny","as_write","as_read"],p.unannotated||"")+'</select></label>'+
      '<label>Code execution<select id="profile-code">'+profileSelectOptions(["","true","false"],codeValue)+'</select></label>'+
      '<label>Management tools<select id="profile-management">'+profileSelectOptions(["","true","false"],managementValue)+'</select></label></div>'+
      '<label class="check-row"><input type="checkbox" id="profile-switchable-explicit"'+(Array.isArray(p.switchable_to)?' checked':'')+'> Manage switchable profiles explicitly</label>'+
      '<label>Switchable profiles<textarea id="profile-switchable" rows="3" placeholder="one-profile-per-line">'+esc(switchable)+'</textarea></label>'+
      '<details><summary>Tool rules</summary><div class="form-grid">'+
      '<label>Allow patterns<textarea id="profile-allow" rows="3" placeholder="github:search_*">'+esc((tools.allow||[]).join("\n"))+'</textarea></label>'+
      '<label>Deny patterns<textarea id="profile-deny" rows="3" placeholder="github:delete_*">'+esc((tools.deny||[]).join("\n"))+'</textarea></label>'+
      '<label>Classifications (JSON object)<textarea id="profile-classify" rows="5" class="mono" placeholder=\'{"server:tool":"read"}\'>'+esc(Object.keys(classify).length?JSON.stringify(classify,null,2):"")+'</textarea></label>'+
      '</div></details>'+
      '<section class="connection-section"><h3>Try policy</h3><label>Tool search<input id="profile-try-query" placeholder="issue search"></label><div id="profile-try-result" class="scope-status muted" role="status" aria-live="polite"></div></section>'+
      '<div class="card-actions"><button type="button" class="secondary" id="profile-try-btn">Try policy</button><button type="submit" class="primary">Save policy</button></div>'+
      '</form>',
      opener
    );
    const buildSafeDraft=()=>{
      const draft={name,servers:[...(p.servers||[])]};
      const title=$("#profile-title").value.trim(); if(title)draft.title=title;
      const description=$("#profile-description").value.trim(); if(description)draft.description=description;
      const maxTier=$("#profile-max-tier").value; if(maxTier)draft.max_tier=maxTier;
      const unannotated=$("#profile-unannotated").value; if(unannotated)draft.unannotated=unannotated;
      const code=$("#profile-code").value; if(code)draft.code_execution=code==="true";
      const management=$("#profile-management").value; if(management)draft.management_tools=management==="true";
      if($("#profile-switchable-explicit").checked){
        draft.switchable_to=$("#profile-switchable").value.split(/\n|,/).map(x=>x.trim()).filter(Boolean);
      }
      const allow=$("#profile-allow").value.split(/\n|,/).map(x=>x.trim()).filter(Boolean);
      const deny=$("#profile-deny").value.split(/\n|,/).map(x=>x.trim()).filter(Boolean);
      const rawClassify=$("#profile-classify").value.trim();
      let classify={};
      if(rawClassify){
        classify=JSON.parse(rawClassify);
        if(!classify||Array.isArray(classify)||typeof classify!=="object")throw new Error("Classifications must be a JSON object.");
      }
      if(allow.length||deny.length||Object.keys(classify).length)draft.tools={allow,deny,classify};
      return draft;
    };
    $("#profile-try-btn").onclick=async()=>{
      const out=$("#profile-try-result");
      try{
        const query=$("#profile-try-query").value.trim();
        if(!query)throw new Error("Enter a tool search first.");
        const result=await api("/api/profiles/try",{method:"POST",body:JSON.stringify({profile:buildSafeDraft(),query,limit:10})});
        out.textContent="Visible results: "+(result.results||[]).length+" · Hidden by profile: "+(result.hidden_by_profile||0);
        out.className="scope-status good-text";
      }catch(e){out.textContent=e.message;out.className="scope-status bad-text"}
    };
    $("#profile-advanced-form").onsubmit=async e=>{
      e.preventDefault();
      try{
        await api("/api/profiles/"+encodeURIComponent(name)+"/advanced",{method:"PUT",body:JSON.stringify(buildSafeDraft())});
        closeModal();toast("Profile policy updated");await load();
      }catch(err){toast(err.message)}
    };
  }catch(e){toast(e.message)}
}

function openNewProfile(){
  openModal('<h2 id="modal-title">New MCPProxy profile</h2><p class="muted">Native profile URL: /mcp/p/&lt;name&gt;. Use lowercase letters, digits, - or _.</p><form id="profile-form" class="form-grid"><label>Name<input id="profile-name" pattern="[a-z0-9][a-z0-9_-]{0,62}" required placeholder="coding"></label><button class="primary" type="submit">Create profile</button></form>');
  $("#profile-form").onsubmit=async e=>{e.preventDefault();try{await api("/api/profiles",{method:"POST",body:JSON.stringify({name:$("#profile-name").value.trim()})});closeModal();toast("Profile created in MCPProxy");await load()}catch(err){toast(err.message)}};
}
function openAssign(profile,opener){
  const p=(state.data?.profiles||[]).find(x=>x.name===profile);
  const assigned=new Set(p?.servers||[]);
  const opts=(state.data?.upstreams||[]).filter(s=>!assigned.has(s.name)).map(s=>'<option value="'+esc(s.name)+'">'+esc(s.name)+'</option>').join("");
  if(!opts){toast("All connections are already assigned to "+profile);return}
  openModal('<h2 id="modal-title">Add connection · '+esc(profile)+'</h2><form id="assign-form" class="form-grid"><label>Connection<select id="assign-server">'+opts+'</select></label><button class="primary" type="submit">Assign</button></form>',opener);
  $("#assign-form").onsubmit=async e=>{e.preventDefault();try{await api("/api/profiles/"+encodeURIComponent(profile)+"/servers",{method:"POST",body:JSON.stringify({server:$("#assign-server").value})});closeModal();toast("Profile updated");await load()}catch(err){toast(err.message)}};
}

async function removeProfileServer(profile,server){
  if(!confirm("Remove "+server+" from "+profile+"?"))return;
  try{await api("/api/profiles/"+encodeURIComponent(profile)+"/servers/"+encodeURIComponent(server),{method:"DELETE",body:"{}"});toast("Profile updated");await load()}catch(e){toast(e.message)}
}
async function deleteProfile(profile){
  if(!confirm("Delete MCPProxy profile "+profile+"? Pinned Agent Tokens are protected and will block this action."))return;
  try{await api("/api/profiles/"+encodeURIComponent(profile),{method:"DELETE",body:"{}"});toast("Profile deleted");await load()}catch(e){toast(e.message)}
}

function openModal(html,opener=document.activeElement){
  state.modalReturnFocus=opener instanceof HTMLElement?opener:null;
  $("#modal-body").innerHTML=html;
  $("#modal").showModal();
  requestAnimationFrame(()=>$("#modal").querySelector("input,select,textarea,button:not(.modal-close),summary")?.focus());
}
function closeModal(){
  const target=state.modalReturnFocus;
  state.modalReturnFocus=null;
  if($("#modal").open)$("#modal").close();
  if(target?.isConnected)target.focus();
}
$("#modal").addEventListener("close",()=>{const target=state.modalReturnFocus;state.modalReturnFocus=null;if(target?.isConnected)target.focus()});

function switchView(name){
  state.view=name;
  $$(".nav-item").forEach(b=>b.classList.toggle("active",b.dataset.view===name));
  $$(".view").forEach(v=>v.classList.toggle("active",v.id==="view-"+name));
  const labels={overview:"Overview",connections:"Connections",profiles:"Profiles",agents:"Agents",activity:"Activity & Security",settings:"Settings"};
  $("#view-title").textContent=labels[name]||name;
}

$("#login-form").onsubmit=async e=>{
  e.preventDefault();
  const input=$("#admin-key");
  setLoginState("checking","Checking MCPProxy…");
  try{
    const r=await api("/api/login",{method:"POST",body:JSON.stringify({key:input.value})});
    state.csrf=r.csrf||"";
    state.mcpproxyVersion=r.mcpproxy_version||"";
    input.value="";
    setLoginState("success","Connected to MCPProxy"+(state.mcpproxyVersion?" · "+state.mcpproxyVersion:""));
    await new Promise(resolve=>setTimeout(resolve,250));
    await load();
  }catch(err){
    setLoginState("error","Connection failed · "+err.message);
    toast(err.message);
  }
};
$("#logout").onclick=async()=>{
  stopLiveUpdates();
  try{await api("/api/logout",{method:"POST",body:"{}"})}catch{}
  state.csrf="";
  $("#app").hidden=true;
  $("#login").hidden=false;
};
$("#refresh").onclick=()=>load({allowEventStart:true});
$("#connection-search").oninput=renderConnections;
$("#new-profile").onclick=openNewProfile;
$("#new-token").onclick=openToken;
$(".modal-close").onclick=closeModal;
$("#modal").addEventListener("click",e=>{if(e.target===$("#modal"))closeModal()});
$$(".nav-item").forEach(b=>b.onclick=()=>switchView(b.dataset.view));
document.addEventListener("visibilitychange",()=>{
  if(document.hidden){
    stopLiveUpdates();
    return;
  }
  if(!$("#app").hidden)load({allowEventStart:true});
});

load({allowEventStart:true});
