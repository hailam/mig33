"""Extract incoming getter evidence using normal-control-flow traversal of javap.
Exception-handler edges and delegated helper method reads are not included.
"""
from pathlib import Path
import re,json
base=Path(__file__).resolve().parents[3] / 'research/applets/v4'
s=(base/'artifacts/bytecode/bc.txt').read_text()
s=s[s.index('  public final synchronized void a(ay[]);'):]
first=s[s.index('lookupswitch'):s.index('}',s.index('lookupswitch'))]
starts={int(op):int(off) for op,off in re.findall(r'^\s+(\d+): (\d+)$',first,re.M)}
lines=[]
switches={}
current_switch=None
for line in s.splitlines():
 m=re.match(r'\s*(\d+): ([a-z][a-z_0-9]*)\s*(.*)',line)
 if m:
  lines.append((int(m[1]),m[2],m[3],line))
  if m[2] in ('lookupswitch','tableswitch'):
   current_switch=int(m[1]);switches[current_switch]=[]
  else:current_switch=None
 elif current_switch is not None:
  m=re.match(r'\s*(?:\d+|default): (\d+)$',line)
  if m:switches[current_switch].append(int(m[1]))
  elif '}' in line:current_switch=None
byoffset={r[0]:r for r in lines}
positions={r[0]:i for i,r in enumerate(lines)}
types={'b':'B','c':'S','d':'I','e':'L','f':'T','g':'X','h':'TA'}
rows=[]
for op,start in sorted(starts.items()):
 seen=set();pending=[start]
 while pending:
  off=pending.pop()
  if off in seen or off>=14677 or off not in byoffset:continue
  seen.add(off)
  offset,ins,args,line=byoffset[off]
  i=positions[off]
  if ins in ('goto','goto_w'):pending.append(int(args.split()[0]))
  elif ins.startswith('if'):pending.extend([int(args.split()[0]),lines[i+1][0]])
  elif ins in ('lookupswitch','tableswitch'):pending.extend(switches[off])
  elif ins not in ('return','areturn','ireturn','athrow'):pending.append(lines[i+1][0])
 block=[r for r in lines if r[0] in seen]
 reads=[]
 for offset,ins,args,line in block:
  m=re.search(r'Method ay\.([bcdefgh]):\(S\)',args)
  if not m:continue
  i=positions[offset];prev=lines[i-1]
  if prev[1].startswith('iconst_'):tag=-1 if prev[1]=='iconst_m1' else int(prev[1][-1])
  elif prev[1] in ('bipush','sipush'):tag=int(prev[2])
  else:continue
  direct=[x[1] for x in lines[max(0,i-4):i-1]]==['aload_1','iload_2','aaload']
  reads.append({'tag':tag,'type':types[m[1]],'bytecode_offset':offset,'direct_response_read':direct})
 fields=sorted({(r['tag'],r['type']) for r in reads if r['direct_response_read']})
 rows.append({'opcode':op,'bytecode_entry':start,'normal_control_flow_offsets':sorted(seen),'direct_fields':[{'tag':t,'type':k} for t,k in fields],'all_getter_reads':reads})
 (base/f'docs/data/protocol-opcode-{op}.txt').write_text('\n'.join(x[3] for x in block)+'\n')
(base/'docs/data/protocol-incoming.json').write_text(json.dumps(rows,indent=2)+'\n')
for row in rows:print(row['opcode'],row['bytecode_entry'],','.join(str(x['tag'])+':'+x['type'] for x in row['direct_fields']))
