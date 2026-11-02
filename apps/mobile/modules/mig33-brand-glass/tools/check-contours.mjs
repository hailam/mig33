import fs from 'node:fs';
const source=fs.readFileSync(process.argv[2],'utf8');
const groups={};
const distance=(a,b)=>Math.hypot(a[0]-b[0],a[1]-b[1]);
const mid=(a,b)=>[(a[0]+b[0])/2,(a[1]+b[1])/2];
function lineDistance(p,a,b){const dx=b[0]-a[0],dy=b[1]-a[1],n=dx*dx+dy*dy;const t=n?Math.max(0,Math.min(1,((p[0]-a[0])*dx+(p[1]-a[1])*dy)/n)):0;return distance(p,[a[0]+t*dx,a[1]+t*dy]);}
function flatten(a,b,c,d,depth=0){if(depth>20||Math.max(lineDistance(b,a,d),lineDistance(c,a,d))<.003)return[a,d];const ab=mid(a,b),bc=mid(b,c),cd=mid(c,d),abc=mid(ab,bc),bcd=mid(bc,cd),m=mid(abc,bcd);return[...flatten(a,ab,abc,m,depth+1).slice(0,-1),...flatten(m,bcd,cd,d,depth+1)];}
for(const match of source.matchAll(/static let (\w+): Path = make\(\[([\s\S]*?)\n  \]\)/g)){
 groups[match[1]]=[...match[2].matchAll(/\[([\d.,\s-]+)\]/g)].map(m=>{const values=m[1].split(',').map(Number);let last=values.slice(0,2),out=[last];for(let i=2;i<values.length;i+=6){const next=values.slice(i+4,i+6);out.push(...flatten(last,values.slice(i,i+2),values.slice(i+2,i+4),next).slice(1));last=next;}if(distance(out[0],out.at(-1))>1e-8)out.push(out[0]);return out;});
}
const cross=(a,b,c)=>(b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0]);
function crosses(a,b,c,d){if(Math.max(a[0],b[0])+1e-8<Math.min(c[0],d[0])||Math.max(c[0],d[0])+1e-8<Math.min(a[0],b[0])||Math.max(a[1],b[1])+1e-8<Math.min(c[1],d[1])||Math.max(c[1],d[1])+1e-8<Math.min(a[1],b[1]))return false;const x=cross(a,b,c),y=cross(a,b,d),u=cross(c,d,a),v=cross(c,d,b);return x*y < -1e-12 && u*v < -1e-12;}
function inside(p,poly){let yes=false;for(let i=0;i<poly.length-1;i++){const a=poly[i],b=poly[i+1];if((a[1]>p[1])!==(b[1]>p[1])&&p[0]<(b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0])yes=!yes;}return yes;}
let errors=0;
for(const[name,paths]of Object.entries(groups)){
 let self=0,inter=0;
 for(const [contour,p] of paths.entries())for(let i=0;i<p.length-1;i++)for(let j=i+2;j<p.length-1;j++){if(i===0&&j===p.length-2)continue;if(crosses(p[i],p[i+1],p[j],p[j+1])){self++;console.log(JSON.stringify({crossing:name,contour,i,j,a:p[i],b:p[i+1],c:p[j],d:p[j+1]}));}}
 for(let p=0;p<paths.length;p++)for(let q=p+1;q<paths.length;q++)for(let i=0;i<paths[p].length-1;i++)for(let j=0;j<paths[q].length-1;j++)if(crosses(paths[p][i],paths[p][i+1],paths[q][j],paths[q][j+1]))inter++;
 const nesting=paths.map((p,index)=>paths.flatMap((q,j)=>j!==index&&inside(p[0],q)?[j]:[]));
 const areas=paths.map(p=>p.slice(0,-1).reduce((sum,a,i)=>sum+a[0]*p[i+1][1]-p[i+1][0]*a[1],0)/2);
 console.log(JSON.stringify({name,contours:paths.length,segments:paths.map(p=>p.length-1),selfCrossings:self,interCrossings:inter,nesting,signedAreas:areas.map(x=>Number(x.toFixed(3)))}));errors+=self+inter;
}
for(const[name,body]of [['mig','cyan'],['number','orange']]){const escaped=groups[name].flat().filter(p=>!inside(p,groups[body][0])).length;console.log(JSON.stringify({name,outsideBodyVertices:escaped}));errors+=escaped;}
process.exitCode=errors?1:0;
