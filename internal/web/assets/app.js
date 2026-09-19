"use strict";

const state={data:null,csrf:"",view:"overview"};
const configuredBase=document.querySelector('meta[name="sidekick-mount-path"]')?.content?.trim()||"";
const BASE=(configuredBase||location.pathname).replace(/\/+$/,"");
const $=(s,r=document)=>r.querySelector(s);
const $$=(s,r=document)=>Array.from(r.querySelectorAll(s));
const esc=v=>String(v??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));

function toast(message){
  const e=$("#toast"); e.textContent=message; e.hidden=false;
  clearTimeout(toast.timer); toast.timer=setTimeout(()=>{e.hidden=true},4200);
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
  if(!r.ok) throw new Error(data.error||("HTTP "+r.status));
  return data;
}

function badge(status,enabled=true,quarantined=false){
  if(quarantined) return '<span class="badge bad">Quarantined</span>';
  if(!enabled) return '<span class="badge warn">Disabled</span>';
  const s=String(status||"unknown").toLowerCase();
  const cls=/ready|connected/.test(s)?"good":(/auth|pending|disabled/.test(s)?"warn":(/error|failed|unhealthy/.test(s)?"bad":""));
  return '<span class="badge '+cls+'">'+esc(status||"unknown")+'</span>';
}

function profileLabel(id){
  const p=(state.data?.profiles||[]).find(x=>x.name===id);
  return p?p.name:(id||"Unassigned");
}

async function load(){
  $("#refresh-state").textContent="Refreshing…";
  try{
    const d=await api("/api/state");
    state.data=d; state.csrf=d.csrf||state.csrf;
    $("#login").hidden=true; $("#app").hidden=false;
    render(); $("#refresh-state").textContent=(d.warnings||[]).length?"Degraded":"Live";
    if((d.warnings||[]).length) toast(d.warnings.join(" · "));
  }catch(e){
    if(/401|authentication/i.test(e.message)){ $("#app").hidden=true; $("#login").hidden=false; }
    else toast(e.message);
    $("#refresh-state").textContent="Offline";
  }
}

function render(){
  renderSummary(); renderProfiles(); renderUpstreams(); renderCredentials(); renderOAuth(); renderTokens(); renderAttention();
}

function renderSummary(){
  const s=state.data?.summary||{};
  const items=[["Upstreams",s.total||0],["Connected",s.connected||0],["Tools",s.tools||0],["Auth needed",s.auth_required||0],["Quarantined",s.quarantined||0]];
  $("#summary").innerHTML=items.map(x=>'<article class="stat"><strong>'+esc(x[1])+'</strong><span>'+esc(x[0])+'</span></article>').join("");
}

function renderProfiles(){
  const ps=state.data?.profiles||[];
  const html=ps.length?ps.map(p=>{
    const servers=p.servers||[];
    const chips=servers.map(n=>'<span class="chip">'+esc(n)+' <button class="chip-x remove-profile-server" type="button" data-profile="'+esc(p.name)+'" data-server="'+esc(n)+'" aria-label="Remove '+esc(n)+'">×</button></span>').join("");
    return '<article class="profile-card"><div class="row"><div><h3>'+esc(p.name)+'</h3><span class="muted">/mcp/p/'+esc(p.name)+'</span></div><span class="badge">'+esc(p.tool_count||0)+' tools</span></div><div class="chips">'+(chips||'<span class="muted">Deny-all until an upstream is assigned</span>')+'</div><div class="card-actions"><button class="secondary assign-server" data-profile="'+esc(p.name)+'">Add upstream</button><button class="ghost delete-profile" data-profile="'+esc(p.name)+'">Delete</button></div></article>';
  }).join(""):'<div class="empty">No MCPProxy profiles yet.</div>';
  $("#profiles-list").innerHTML=html; $("#overview-profiles").innerHTML=html;
  $$(".assign-server").forEach(b=>b.onclick=()=>openAssign(b.dataset.profile));
  $$(".remove-profile-server").forEach(b=>b.onclick=()=>removeProfileServer(b.dataset.profile,b.dataset.server));
  $$(".delete-profile").forEach(b=>b.onclick=()=>deleteProfile(b.dataset.profile));
}

function serverCard(s,withActions=true){
  let action="";
  if(withActions){
    action=s.oauth?'<button class="secondary oauth-start" data-name="'+esc(s.name)+'">'+(s.authenticated?"Reconnect OAuth":"Connect OAuth")+'</button>':'<button class="secondary credential-open" data-name="'+esc(s.name)+'">Set credential</button>';
    if(s.name==="omniroute" && state.data?.capabilities?.omniroute_restore_master){ action+='<button class="secondary omni-restore">Restore existing Master</button>'; }
  }
  const memberships=(s.profiles||[]).join(", ")||"Unassigned";
  return '<article class="server-card" data-name="'+esc(s.name)+'"><div class="server-top"><div><h3>'+esc(s.name)+'</h3><span class="muted">'+esc(memberships)+'</span></div>'+badge(s.status,s.enabled,s.quarantined)+'</div><div class="server-meta"><span>'+esc(s.protocol||"MCP")+'</span><strong>'+esc(s.tool_count||0)+' tools</strong></div><div class="preview">'+(s.credential_configured?esc(s.credential_preview||"Configured"):"No credential detected")+'</div>'+(withActions?'<div class="card-actions">'+action+'</div>':"")+'</article>';
}

function renderUpstreams(){
  const q=($("#upstream-search")?.value||"").toLowerCase();
  const list=(state.data?.upstreams||[]).filter(s=>!q||s.name.toLowerCase().includes(q)||String(s.status).toLowerCase().includes(q));
  $("#upstreams-list").innerHTML=list.length?list.map(s=>serverCard(s,true)).join(""):'<div class="empty">No matching upstreams.</div>';
  wireServerActions($("#upstreams-list"));
}
function renderCredentials(){
  const list=(state.data?.upstreams||[]).filter(s=>!s.oauth);
  $("#credentials-list").innerHTML=list.length?list.map(s=>serverCard(s,true)).join(""):'<div class="empty">No API-key upstreams detected.</div>';
  wireServerActions($("#credentials-list"));
}
function renderOAuth(){
  const list=(state.data?.upstreams||[]).filter(s=>s.oauth);
  $("#oauth-list").innerHTML=list.length?list.map(s=>serverCard(s,true)).join(""):'<div class="empty">No OAuth upstreams detected.</div>';
  wireServerActions($("#oauth-list"));
}
function renderAttention(){
  const warnings=state.data?.warnings||[];
  const list=(state.data?.upstreams||[]).filter(s=>!s.enabled||s.quarantined||!/ready|connected/i.test(s.status||""));
  const warningHtml=warnings.map(w=>'<div class="token-row"><div class="token-info"><h3>Sidekick dependency</h3><p>'+esc(w)+'</p></div><span class="badge warn">Degraded</span></div>').join("");
  const upstreamHtml=list.map(s=>'<div class="token-row"><div class="token-info"><h3>'+esc(s.name)+'</h3><p>'+esc(s.status||"unknown")+' · '+(s.tool_count||0)+' tools</p></div>'+badge(s.status,s.enabled,s.quarantined)+'</div>').join("");
  $("#attention").innerHTML=warningHtml+upstreamHtml||(warnings.length||list.length?"":'<div class="empty">Everything looks healthy.</div>');
}

function wireServerActions(root){
  $$(".oauth-start",root).forEach(b=>b.onclick=()=>startOAuth(b.dataset.name));
  $$(".credential-open",root).forEach(b=>b.onclick=()=>openCredential(b.dataset.name));
  $$(".omni-restore",root).forEach(b=>b.onclick=restoreOmniRouteMaster);
}

async function restoreOmniRouteMaster(){
  try{
    await api("/api/adapters/omniroute/restore-master",{method:"POST",body:"{}"});
    toast("OmniRoute Master assigned to MCPProxy");
    await load();
  }catch(e){toast(e.message)}
}

async function startOAuth(name){
  const popup=window.open("about:blank","mcp-oauth-"+name);
  if(!popup){ toast("Popup blocked. Allow popups for this site and retry."); return; }
  try{ popup.document.title="Starting OAuth"; popup.document.body.textContent="Starting a fresh OAuth session…"; }catch{}
  try{
    const result=await api("/api/upstreams/"+encodeURIComponent(name)+"/oauth/start",{method:"POST",body:"{}"});
    popup.location=result.browser_url; popup.focus(); toast("Fresh OAuth session started for "+name+". Complete sign-in in the new tab."); setTimeout(load,2500);
  }catch(e){
    try{popup.document.body.textContent="OAuth start failed: "+e.message}catch{}
    toast("OAuth start failed: "+e.message);
  }
}

function openCredential(name){
  openModal('<h2>Credential · '+esc(name)+'</h2><p class="muted">The full secret is submitted once and is not returned by Sidekick afterward.</p><form id="credential-form" class="form-grid"><label>Authentication format<select id="cred-mode"><option value="bearer">Authorization: Bearer</option><option value="x-api-key">X-API-Key</option><option value="custom-header">Custom header</option></select></label><label id="header-label" hidden>Header name<input id="cred-header" placeholder="X-Custom-Key"></label><label>API key / token<input id="cred-value" type="password" autocomplete="off" required></label><button class="primary" type="submit">Save / replace & enable</button></form>');
  $("#cred-mode").onchange=e=>{$("#header-label").hidden=e.target.value!=="custom-header"};
  $("#credential-form").onsubmit=async e=>{
    e.preventDefault();
    try{
      await api("/api/upstreams/"+encodeURIComponent(name)+"/credential",{method:"POST",body:JSON.stringify({mode:$("#cred-mode").value,header_name:$("#cred-header").value,value:$("#cred-value").value})});
      $("#modal").close(); toast(name+" credential updated"); await load();
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
function showOneTimeToken(name,token){
  openModal('<h2>Save this token now</h2><p class="muted">MCPProxy shows the secret only once.</p><label>'+esc(name)+'<input id="one-token" readonly value="'+esc(token)+'"></label><div class="card-actions"><button id="copy-token" class="primary">Copy token</button></div>');
  $("#copy-token").onclick=async()=>{await navigator.clipboard.writeText($("#one-token").value);toast("Token copied")};
}

function openToken(){
  const upstreams=state.data?.upstreams||[],profiles=state.data?.profiles||[];
  const profileOpts=profiles.map(p=>'<option value="'+esc(p.name)+'">'+esc(p.name)+'</option>').join("");
  const servers='<label class="chip"><input type="checkbox" id="server-all" value="*"> All upstreams (*)</label>'+upstreams.map(s=>'<label class="chip"><input type="checkbox" name="server" value="'+esc(s.name)+'"> '+esc(s.name)+'</label>').join("");
  openModal('<h2>New Agent Token</h2><form id="token-form" class="form-grid"><label>Name<input id="token-name" pattern="[A-Za-z0-9][A-Za-z0-9_-]*" required placeholder="family-agent"></label><label>Profile pin<select id="token-profile"><option value="">No profile pin</option>'+profileOpts+'</select></label><fieldset><legend>Allowed upstreams</legend><div class="chips">'+servers+'</div><small class="muted">With a profile pin, Sidekick uses * and MCPProxy enforces the profile server-side.</small></fieldset><fieldset><legend>Permissions</legend><div class="chips"><label class="chip"><input type="checkbox" name="perm" value="read" checked> read</label><label class="chip"><input type="checkbox" name="perm" value="write"> write</label><label class="chip"><input type="checkbox" name="perm" value="destructive"> destructive</label></div></fieldset><label>Expiry<input id="token-expiry" value="30d" placeholder="30d"></label><button class="primary" type="submit">Create token</button></form>');
  const syncTokenScope=()=>{
    const pinned=!!$("#token-profile").value,all=$("#server-all");
    all.checked=pinned||all.checked;
    $$('input[name="server"]').forEach(x=>{x.disabled=pinned||all.checked;if(pinned||all.checked)x.checked=false});
  };
  $("#token-profile").onchange=()=>{if(!$("#token-profile").value)$("#server-all").checked=false;syncTokenScope()};
  $("#server-all").onchange=syncTokenScope;
  $("#token-form").onsubmit=async e=>{
    e.preventDefault();
    const serversChosen=$("#server-all").checked?["*"]:$$('input[name="server"]:checked').map(x=>x.value);
    const perms=$$('input[name="perm"]:checked').map(x=>x.value);
    const destructive=perms.includes("destructive");
    const confirmation=!destructive||confirm("This token can call destructive tools. Grant destructive permission?");
    if(!confirmation)return;
    try{
      const r=await api("/api/tokens",{method:"POST",body:JSON.stringify({name:$("#token-name").value,allowed_servers:serversChosen,permissions:perms,expires_in:$("#token-expiry").value,profile_pin:$("#token-profile").value,confirm_destructive:confirmation})});
      showOneTimeToken(r.name,r.token);await load();
    }catch(err){toast(err.message)}
  };
}

function openNewProfile(){
  openModal('<h2>New MCPProxy profile</h2><p class="muted">Native profile URL: /mcp/p/&lt;name&gt;. Use lowercase letters, digits, - or _.</p><form id="profile-form" class="form-grid"><label>Name<input id="profile-name" pattern="[a-z0-9][a-z0-9_-]{0,62}" required placeholder="coding"></label><button class="primary" type="submit">Create profile</button></form>');
  $("#profile-form").onsubmit=async e=>{e.preventDefault();try{await api("/api/profiles",{method:"POST",body:JSON.stringify({name:$("#profile-name").value.trim()})});$("#modal").close();toast("Profile created in MCPProxy");await load()}catch(err){toast(err.message)}};
}
function openAssign(profile){
  const p=(state.data?.profiles||[]).find(x=>x.name===profile);
  const assigned=new Set(p?.servers||[]);
  const opts=(state.data?.upstreams||[]).filter(s=>!assigned.has(s.name)).map(s=>'<option value="'+esc(s.name)+'">'+esc(s.name)+'</option>').join("");
  if(!opts){toast("All upstreams are already assigned to "+profile);return}
  openModal('<h2>Add upstream · '+esc(profile)+'</h2><form id="assign-form" class="form-grid"><label>Upstream<select id="assign-server">'+opts+'</select></label><button class="primary" type="submit">Assign</button></form>');
  $("#assign-form").onsubmit=async e=>{e.preventDefault();try{await api("/api/profiles/"+encodeURIComponent(profile)+"/servers",{method:"POST",body:JSON.stringify({server:$("#assign-server").value})});$("#modal").close();toast("Profile updated");await load()}catch(err){toast(err.message)}};
}

async function removeProfileServer(profile,server){
  if(!confirm("Remove "+server+" from "+profile+"?"))return;
  try{await api("/api/profiles/"+encodeURIComponent(profile)+"/servers/"+encodeURIComponent(server),{method:"DELETE",body:"{}"});toast("Profile updated");await load()}catch(e){toast(e.message)}
}
async function deleteProfile(profile){
  if(!confirm("Delete MCPProxy profile "+profile+"? Pinned Agent Tokens are protected and will block this action."))return;
  try{await api("/api/profiles/"+encodeURIComponent(profile),{method:"DELETE",body:"{}"});toast("Profile deleted");await load()}catch(e){toast(e.message)}
}
function openModal(html){$("#modal-body").innerHTML=html;$("#modal").showModal()}
function switchView(name){
  state.view=name;
  $$(".nav-item").forEach(b=>b.classList.toggle("active",b.dataset.view===name));
  $$(".view").forEach(v=>v.classList.toggle("active",v.id==="view-"+name));
  const labels={overview:"Overview",profiles:"Profiles",upstreams:"Upstreams",credentials:"Credentials",oauth:"OAuth",tokens:"Agent Tokens",security:"Security",setup:"Setup / Restore"};
  $("#view-title").textContent=labels[name]||name;
}

$("#login-form").onsubmit=async e=>{
  e.preventDefault();
  try{const r=await api("/api/login",{method:"POST",body:JSON.stringify({key:$("#admin-key").value})});state.csrf=r.csrf||"";$("#admin-key").value="";await load()}catch(err){toast(err.message)}
};
$("#logout").onclick=async()=>{try{await api("/api/logout",{method:"POST",body:"{}"})}catch{}state.csrf="";$("#app").hidden=true;$("#login").hidden=false};
$("#refresh").onclick=load;
$("#upstream-search").oninput=renderUpstreams;
$("#new-profile").onclick=openNewProfile;
$("#new-token").onclick=openToken;
$(".modal-close").onclick=()=>$("#modal").close();
$("#modal").addEventListener("click",e=>{if(e.target===$("#modal"))$("#modal").close()});
$$(".nav-item").forEach(b=>b.onclick=()=>switchView(b.dataset.view));

load();
setInterval(()=>{if(!$("#app").hidden)load()},15000);
