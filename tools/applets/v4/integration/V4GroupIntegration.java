import java.lang.reflect.*;
import java.util.*;
import java.nio.file.*;
import javax.microedition.lcdui.*;
import javax.microedition.io.Connector;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;
import org.microemu.MIDletBridge;

/** Four unchanged original-client JVMs, coordinated only by local test marker files. */
public class V4GroupIntegration extends V4Integration {
 static final Path groupRoot=Paths.get("data/applets/integration/group-out");
 static String group;
 interface Action { void run() throws Exception; }
 static void until(String name,java.util.concurrent.Callable<Boolean> check)throws Exception {
  long deadline=System.currentTimeMillis()+60000;
  while(System.currentTimeMillis()<deadline){if(check.call()){System.out.println("PASS "+name);return;}Thread.sleep(100);}
  try{save("failure");}catch(Exception ignored){}
  throw new AssertionError("Timeout "+user+": "+name);
 }
 static void mark(String name)throws Exception{Files.writeString(out.resolve(name),"ok\n");}
 static void peerMark(String peer,String name)throws Exception{until(peer+" reached "+name,()->Files.exists(groupRoot.resolve(peer).resolve(name)));}
 static int transaction(String direction,int opcode){synchronized(Connector.events){for(int i=Connector.events.size()-1;i>=0;i--){String[] e=Connector.events.get(i).split("\t");if(e.length>=4&&e[0].equals(direction)&&e[1].equals(""+opcode))return Integer.parseInt(e[2]);}}return -1;}
 static boolean received(int opcode,int transaction){synchronized(Connector.events){for(String e:Connector.events)if(e.startsWith("RX\t"+opcode+"\t"+transaction+"\t"))return true;}return false;}
 static String responseText(int opcode,int tx,short tag)throws Exception {String file=null;synchronized(Connector.events){for(String e:Connector.events){String[] parts=e.split("\t");if(parts.length>=5&&parts[0].equals("RX")&&parts[1].equals(""+opcode)&&parts[2].equals(""+tx))file=parts[4];}}if(file==null)throw new AssertionError("Missing captured response");Class<?> packet=Class.forName("ay");Object[] parsed=(Object[])method(packet,"a",java.lang.reflect.Array.newInstance(packet,0).getClass(),byte[].class).invoke(null,Files.readAllBytes(out.resolve(file)));return (String)method(packet,"f",String.class,short.class).invoke(parsed[0],tag);}
 static int request(int opcode,int response,Action action)throws Exception {
  int before=Connector.count("TX",opcode);action.run();until("original builder transmitted "+opcode,()->Connector.count("TX",opcode)>before);int tx=transaction("TX",opcode);
  until("original "+opcode+" received correlated "+response,()->received(response,tx));return tx;
 }
 static Object tabs()throws Exception{return get(canvas,"Y");}
 static int groupTab()throws Exception {Object ts=tabs();Vector v=(Vector)field(ts.getClass(),"a",Vector.class).get(ts);for(int i=0;i<v.size();i++){if((Integer)method(ts.getClass(),"i",int.class,int.class).invoke(ts,i)==4){Object chat=method(ts.getClass(),"c",Class.forName("s"),int.class).invoke(ts,i);String id=(String)field(chat.getClass(),"b",String.class).get(chat);if(group==null||group.equals(id))return i;}}return -1;}
 static Object groupChat()throws Exception{int i=groupTab();return i<0?null:method(tabs().getClass(),"c",Class.forName("s"),int.class).invoke(tabs(),i);}
 static String chatRows()throws Exception{Object c=groupChat();if(c==null)return "";Vector rows=(Vector)field(c.getClass(),"y",Vector.class).get(c);StringBuilder b=new StringBuilder();synchronized(rows){for(Object row:rows)b.append(row).append('\n');}return b.toString();}
 static void selectGroup()throws Exception {int tab=groupTab();if(tab<0)throw new AssertionError("No original group tab");method(tabs().getClass(),"g",void.class,int.class).invoke(tabs(),tab);}
 static void message(String text)throws Exception{request(500,1,()->method(bc,"a",void.class,short.class,String.class,String.class,int.class,byte[].class,short.class,int.class).invoke(network,(short)1,group,text,2,null,(short)1,-1));}
 static void expectMessage(String phrase,String capture)throws Exception{until("original group chat contains "+phrase,()->chatRows().contains(phrase));selectGroup();save(capture);Files.writeString(out.resolve(capture+"-model.txt"),chatRows());}
 static void privateTab(String peer)throws Exception{Object roster=get(canvas,"Z");int rows=(Integer)method(roster.getClass(),"c",int.class).invoke(roster);for(int i=0;i<rows;i++){Object contact=method(roster.getClass(),"e",Class.forName("t"),int.class).invoke(roster,i);if(contact!=null&&peer.equals(field(contact.getClass(),"e",String.class).get(contact))){method(tabs().getClass(),"a",void.class,Class.forName("t"),int.class,int.class).invoke(tabs(),contact,1,0);return;}}throw new AssertionError("Missing roster contact "+peer);}
 static void participants(String name,String... expected)throws Exception {
  selectGroup();method(cc,"k",void.class).invoke(canvas);
  request(754,755,()->method(bc,"b",void.class,String.class,byte.class).invoke(network,group,(byte)1));
  until("original participant popup constructed",()->method(cc,"i",Class.forName("y")).invoke(canvas)!=null);
  Object popup=method(cc,"i",Class.forName("y")).invoke(canvas);int count=(Integer)method(popup.getClass(),"c",int.class).invoke(popup);TreeSet<String> names=new TreeSet<>();for(int i=0;i<count;i++)names.add((String)method(popup.getClass(),"a",String.class,int.class).invoke(popup,i));
  if(count!=expected.length||!names.equals(new TreeSet<>(Arrays.asList(expected))))throw new AssertionError("Participant UI mismatch "+names+" expected "+Arrays.toString(expected));
  save(name);Files.writeString(out.resolve(name+"-model.txt"),String.join("\n",names)+"\n");System.out.println("PASS original participant UI "+names);method(cc,"k",void.class).invoke(canvas);
 }
 static void deny(String marker)throws Exception {
  request(500,0,()->method(bc,"a",void.class,short.class,String.class,String.class,int.class,byte[].class,short.class,int.class).invoke(network,(short)1,group,"DENIED "+marker,2,null,(short)1,-1));
  Thread.sleep(200);if(groupTab()>=0)selectGroup();save(marker);method(cc,"j",void.class).invoke(canvas);mark(marker);
 }
 static void init()throws Exception {init(true);}
 static void init(boolean privateTabs)throws Exception {
  Files.createDirectories(out);Files.deleteIfExists(out.resolve("events.tsv"));System.setProperty("java.awt.headless","true");System.setProperty("microedition.platform","Loopback group integration MicroEmulator");
  try{Connector.open("http://integration-proof.invalid");throw new AssertionError("Remote connection allowed");}catch(javax.microedition.io.ConnectionNotFoundException expected){}
  if(!Connector.class.getProtectionDomain().getCodeSource().getLocation().toString().contains("data/applets/build/integration/classes"))throw new AssertionError("Wrong Connector bridge");
  Headless h=new Headless();Common emu=(Common)get(h,"emulator");org.microemu.EmulatorContext ctx=(org.microemu.EmulatorContext)get(h,"context");
  emu.initParams(new ArrayList<String>(Arrays.asList("--rms","memory","--appclassloader","system","main.Main")),new org.microemu.app.util.DeviceEntry("Default device",null,DeviceImpl.DEFAULT_LOCATION,true,false),J2SEDevice.class);
  J2SEDeviceDisplay dd=(J2SEDeviceDisplay)ctx.getDeviceDisplay();dd.setDisplayRectangle(new Rectangle(0,0,240,320));dd.setDisplayPaintable(new Rectangle(0,0,240,320));emu.initMIDlet(true);Thread.sleep(1600);
  javax.microedition.midlet.MIDlet midlet=MIDletBridge.getCurrentMIDlet();canvas=Display.getDisplay(midlet).getCurrent();cc=Class.forName("aj");bc=Class.forName("bc");Display.getDisplay(midlet).setCurrent(new Canvas(){public void paint(Graphics g){}});Thread.sleep(100);
  field(cc,"c",String.class).set(null,"4.62");field(cc,"a",String.class).set(null,".000");field(cc,"b",int.class).setInt(null,transport.equals("http")?1:2);field(bc,"e",String.class).set(null,"127.0.0.1");field(bc,"f",String.class).set(null,"9119");field(bc,"d",String.class).set(null,"http://127.0.0.1:8080/");
  method(cc,"c",void.class).invoke(canvas);network=field(cc,"ab",bc).get(canvas);method(cc,"a",void.class,String.class).invoke(canvas,"Connecting...");method(cc,"f",void.class,String.class,String.class).invoke(canvas,user,user+"-demo");
  until("original "+transport+" login and roster",()->Connector.count("TX",200)>0&&Connector.count("RX",203)>0&&Connector.count("RX",403)>0&&Connector.count("RX",715)>0);Thread.sleep(400);
  if(privateTabs){if(user.equals("alice"))privateTab("bob");if(user.equals("bob"))privateTab("alice");}mark("ready");
 }
 static void alice()throws Exception {
  for(String peer:new String[]{"bob","carol","dave"})peerMark(peer,"ready");
  int created=request(751,750,()->method(bc,"a",void.class,String.class,String.class,String.class,byte.class).invoke(network,"bob","carol",null,(byte)1));group=responseText(750,created,(short)1);
  until("original private tab converted into group",()->groupTab()>=0);Files.writeString(groupRoot.resolve("group-id.tmp"),group);Files.move(groupRoot.resolve("group-id.tmp"),groupRoot.resolve("group-id.txt"),StandardCopyOption.ATOMIC_MOVE);selectGroup();save("created-group");participants("participants-three","alice","bob","carol");
  message("Phase1 hello from Alice");for(String peer:new String[]{"bob","carol"})peerMark(peer,"phase1-received");expectMessage("Bob group reply","group-phase1");peerMark("carol","bob-reply-received");peerMark("dave","outsider-denied");
  request(752,1,()->method(bc,"a",void.class,String.class,String.class,String.class,byte.class).invoke(network,null,"dave",group,(byte)1));participants("participants-four","alice","bob","carol","dave");mark("invited-dave");message("Phase2 hello from Alice");
  for(String peer:new String[]{"bob","carol","dave"})peerMark(peer,"phase2-received");expectMessage("Dave joined reply","group-four-members");
  for(String peer:new String[]{"bob","carol"})peerMark(peer,"dave-reply-received");mark("may-leave");peerMark("carol","postleave-denied");participants("participants-after-leave","alice","bob","dave");message("After Carol left");
  for(String peer:new String[]{"bob","dave"})peerMark(peer,"afterleave-received");mark("final-sent");peerMark("carol","no-postleave-delivery");mark("finished");
 }
 static void member()throws Exception {
  until("group ID available",()->Files.exists(groupRoot.resolve("group-id.txt")));group=Files.readString(groupRoot.resolve("group-id.txt"));
  if(user.equals("dave")){
   deny("outsider-denied");peerMark("alice","invited-dave");expectMessage("Phase2 hello","group-after-invite");mark("phase2-received");message("Dave joined reply");expectMessage("After Carol left","group-after-carol-leaves");mark("afterleave-received");
  }else{
   expectMessage("Phase1 hello","group-phase1");mark("phase1-received");if(user.equals("bob"))message("Bob group reply");else{expectMessage("Bob group reply","group-bob-reply");mark("bob-reply-received");}expectMessage("Phase2 hello","group-four-members");mark("phase2-received");expectMessage("Dave joined reply","group-dave-reply");mark("dave-reply-received");
   if(user.equals("carol")){
    peerMark("alice","may-leave");request(753,1,()->method(bc,"a",void.class,String.class,byte.class).invoke(network,group,(byte)1));deny("postleave-denied");peerMark("alice","final-sent");
    int polls=Connector.emptyPolls;until("post-leave HTTP polling continued",()->Connector.emptyPolls>polls);if(chatRows().contains("After Carol left"))throw new AssertionError("Former member received post-leave text");mark("no-postleave-delivery");
   }else{expectMessage("After Carol left","group-after-carol-leaves");mark("afterleave-received");}
  }
  peerMark("alice","finished");
 }
 public static void main(String[] args){try{user=args[0];transport=args[1];out=groupRoot.resolve(user);Connector.out=out;init();if(user.equals("alice"))alice();else member();logout();Files.writeString(out.resolve("result.txt"),"PASS original v4.62 group integration: "+user+" over "+transport+"; group "+group+". See runtime.log, binary frames, model text and original Canvas screenshots.\n");System.out.println("PASS "+user+" group integration complete");System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
}
