import http from "node:http";

const port = Number(process.argv[2] || 18080);
let profiles = [{name:"personal",servers:["github"],tool_count:11}];
let tokens = [];
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
    if(req.method==="GET" && url.pathname==="/api/v1/servers")
      return json(res,200,{success:true,data:{servers}});
    if(req.method==="GET" && url.pathname==="/api/v1/profiles")
      return json(res,200,{success:true,data:{profiles}});
    if(req.method==="GET" && url.pathname==="/api/v1/tokens")
      return json(res,200,{success:true,data:{tokens}});
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
