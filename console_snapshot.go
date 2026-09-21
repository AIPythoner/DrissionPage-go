package drissionpage

// Accessors are represented without invoking them. Cycles and depth limits are
// explicit so console collection remains finite for arbitrary application data.
const consoleSnapshotJS = `function(){
 const seen=new WeakSet();
 function copy(v,depth){
  if(v===null)return null;
  if(typeof v==='bigint'||typeof v==='symbol'||typeof v==='function'||typeof v==='undefined')return String(v);
  if(typeof v!=='object')return Number.isFinite(v)||typeof v!=='number'?v:String(v);
  if(seen.has(v))return '[Circular]'; if(depth>=8)return '[MaxDepth]'; seen.add(v);
  const out=Array.isArray(v)?[]:Object.create(null), descriptors=Object.getOwnPropertyDescriptors(v);
  let count=0; for(const key of Object.keys(descriptors)){if(Array.isArray(v)&&key==='length')continue;if(++count>1000){out['[Truncated]']=true;break;}const d=descriptors[key];out[key]='value'in d?copy(d.value,depth+1):'[Accessor]';}
  seen.delete(v);return out;
 }
 return copy(this,0);
}`
