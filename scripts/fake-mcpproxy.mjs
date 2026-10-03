import http from "node:http";

const port = Number(process.argv[2] || 18080);
let profiles = [{name:"personal",servers:["github"],tool_count:11}];
let tokens = [];
let eventConnections = 0;
const servers = [
  {name:"github",enabled:true,status:"ready",protocol:"http",tool_count:11,headers:{Authorization:"Bearer fake-secret"}},
  {name:"filesystem",enabled:true,status:"ready",protocol:"stdio",tool_count:7},
  {name:"google-oauth",enabled:true,status:"ready",protocol:"http",tool_count:5,authenticated:true,oauth:{provider:"google"}},
];

const json=(res,status,body)=>{
  res.writeHead(status,{"content-type":"application/json"});
  res.end(JSON.stringify(body));
};
const readBody=req=>new Promise((resolve,reject)=>{
  let data="";
  req.on("data",chunk=>data+=chunk);
  req.on("end",()=>{try{resolve(data?JSON.parse(data):{})}catch(e){reject(e)}});
  req.on("error",reject);
});

const server=http.createServer(async(req,res)=>{
  const url=new URL(req.url,"http://127.0.0.1");
  try {
    if(req.method==="GET" && url.pathname==="/api/v1/info")
      return json(res,200,{success:true,data:{version:"v0.69.0"}});
    if(req.method==="GET" && url.pathname==="/events"){
      eventConnections++;
      if(req.headers["x-api-key"]!=="admin-key") return json(res,401,{error:"unauthorized"});
      res.writeHead(200,{"content-type":"text/event-stream","cache-control":"no-cache","connection":"keep-alive"});
      res.write('event: servers.changed\ndata: {"payload":{"reason":"smoke","secret":"SUPER_SECRET_SSE_SMOKE"}}\n\n');
      if(eventConnections===1){
        setTimeout(()=>res.end(),250);
        return;
      }
      const timer=setInterval(()=>res.write("event: ping\ndata: {}\n\n"),5000);
      req.on("close",()=>clearInterval(timer));
      return;
    }
    if(req.method==="GET" && url.pathname==="/api/v1/servers")
      return json(res,200,{success:true,data:{servers}});
    if(req.method==="GET" && url.pathname==="/api/v1/profiles")
      return json(res,200,{success:true,data:{profiles}});
    if(req.method==="GET" && url.pathname==="/api/v1/profiles/try"){
      res.writeHead(405,{"content-type":"application/json"});
      return res.end(JSON.stringify({error:"method not allowed"}));
    }
    const profileDetail=url.pathname.match(/^\/api\/v1\/profiles\/([^/]+)$/);
    if(req.method==="GET" && profileDetail){
      const profile=profiles.find(p=>p.name===decodeURIComponent(profileDetail[1]));
      return profile?json(res,200,{success:true,data:profile}):json(res,404,{error:"profile not found"});
    }
    if(req.method==="PUT" && profileDetail){
      const name=decodeURIComponent(profileDetail[1]);
      const body=await readBody(req);
      const index=profiles.findIndex(p=>p.name===name);
      if(index<0)return json(res,404,{error:"profile not found"});
      profiles[index]={...profiles[index],...body,name};
      return json(res,200,{success:true,data:{profile:profiles[index],warnings:[]}});
    }
    if(req.method==="POST" && url.pathname==="/api/v1/profiles/try"){
      await readBody(req);
      return json(res,200,{success:true,data:{results:[{score:1,tool:{name:"github:search_code",server_name:"github"}}],hidden_by_profile:1,hidden:[]}});
    }
    if(req.method==="GET" && url.pathname==="/api/v1/tokens")
      return json(res,200,{success:true,data:{tokens}});
    if(req.method==="POST" && url.pathname==="/api/v1/tokens"){
      const body=await readBody(req);
      const created={
        name:body.name,token:"mcp_agt_smoke_once",token_prefix:"mcp_agt_smok",
        allowed_servers:body.allowed_servers||["*"],permissions:body.permissions||["read"],
        profile_pin:body.profile_pin||"",expires_at:"2099-01-01T00:00:00Z",created_at:"2026-10-03T00:00:00Z"
      };
      tokens=[...tokens,{...created,token:undefined,revoked:false}];
      return json(res,201,{success:true,data:created});
    }
    if(req.method==="PATCH" && url.pathname==="/api/v1/config"){
      const body=await readBody(req);
      if(!Array.isArray(body.profiles)) return json(res,400,{error:"profiles required"});
      profiles=body.profiles.map(p=>({name:p.name,servers:[...(p.servers||[])],tool_count:(p.servers||[]).length*7}));
      return json(res,200,{success:true,data:{profiles}});
    }
    const logout=url.pathname.match(/^\/api\/v1\/servers\/([^/]+)\/logout$/);
    if(req.method==="POST" && logout) return json(res,200,{success:true});
    const login=url.pathname.match(/^\/api\/v1\/servers\/([^/]+)\/login$/);
    if(req.method==="POST" && login)
      return json(res,200,{success:true,data:{auth_url:"https://accounts.example.invalid/oauth",correlation_id:"smoke-oauth"}});
    return json(res,404,{error:"not found",path:url.pathname});
  } catch(err) {
    return json(res,500,{error:String(err)});
  }
});
server.listen(port,"127.0.0.1",()=>console.log("fake MCPProxy listening on "+port));
