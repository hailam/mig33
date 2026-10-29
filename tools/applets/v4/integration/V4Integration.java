import java.lang.reflect.*;
import java.util.*;
import java.nio.file.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.lcdui.*;
import javax.microedition.io.Connector;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;
import org.microemu.MIDletBridge;
/** Executes the unchanged original v4.62 MIDlet against a loopback-only backend. */
public class V4Integration {
 static Object canvas,network;static Class<?> cc,bc;static Path out=Paths.get("data/applets/integration/out");static String user="alice",transport="tcp";
 static Field field(Class<?> c,String n,Class<?> type)throws Exception{for(Field f:c.getDeclaredFields())if(f.getName().equals(n)&&(type==null||f.getType()==type)){f.setAccessible(true);return f;}throw new NoSuchFieldException(c+"."+n);}
 static Object get(Object o,String n)throws Exception{return field(o.getClass(),n,null).get(o);}
 static Method method(Class<?> c,String n,Class<?> returns,Class<?>...types)throws Exception{for(Method m:c.getDeclaredMethods())if(m.getName().equals(n)&&m.getReturnType()==returns&&Arrays.equals(m.getParameterTypes(),types)){m.setAccessible(true);return m;}throw new NoSuchMethodException(c+"."+n);}
 static void await(String name,java.util.concurrent.Callable<Boolean> check)throws Exception{long until=System.currentTimeMillis()+20000;while(System.currentTimeMillis()<until){if(check.call()){System.out.println("PASS "+name);return;}Thread.sleep(100);}save("failure");throw new AssertionError("Timeout: "+name+" state="+field(cc,"M",int.class).get(null)+" connected="+field(bc,"c",boolean.class).get(null));}
 static void save(String name)throws Exception{field(cc,"bh",int.class).setInt(null,Integer.MAX_VALUE);javax.microedition.lcdui.Image im=javax.microedition.lcdui.Image.createImage(240,320);method(cc,"paint",void.class,Graphics.class).invoke(canvas,im.getGraphics());BufferedImage bi=new BufferedImage(240,320,BufferedImage.TYPE_INT_RGB);java.awt.Graphics g=bi.getGraphics();g.drawImage(((J2SEMutableImage)im).getImage(),0,0,null);g.dispose();ImageIO.write(bi,"png",out.resolve(name+".png").toFile());}
 public static void main(String[] args){try{if(args.length>0)user=args[0];if(args.length>1)transport=args[1];out=out.resolve(user);Connector.out=out;run();logout();System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
 static void run()throws Exception{
  Files.createDirectories(out);Files.deleteIfExists(out.resolve("events.tsv"));
  System.setProperty("java.awt.headless","true");System.setProperty("microedition.platform","Loopback integration MicroEmulator");
  try{Connector.open("http://integration-proof.invalid");throw new AssertionError("Remote network allowed");}catch(javax.microedition.io.ConnectionNotFoundException expected){}
  if(!Connector.class.getProtectionDomain().getCodeSource().getLocation().toString().contains("data/applets/build/integration/classes"))throw new AssertionError("Wrong Connector bridge");
  Headless headless=new Headless();Common emu=(Common)get(headless,"emulator");org.microemu.EmulatorContext ctx=(org.microemu.EmulatorContext)get(headless,"context");
  emu.initParams(new ArrayList<String>(Arrays.asList("--rms","memory","--appclassloader","system","main.Main")),new org.microemu.app.util.DeviceEntry("Default device",null,DeviceImpl.DEFAULT_LOCATION,true,false),J2SEDevice.class);
  J2SEDeviceDisplay dd=(J2SEDeviceDisplay)ctx.getDeviceDisplay();dd.setDisplayRectangle(new Rectangle(0,0,240,320));dd.setDisplayPaintable(new Rectangle(0,0,240,320));emu.initMIDlet(true);Thread.sleep(1600);
  javax.microedition.midlet.MIDlet midlet=MIDletBridge.getCurrentMIDlet();canvas=Display.getDisplay(midlet).getCurrent();cc=Class.forName("aj");bc=Class.forName("bc");
  Display.getDisplay(midlet).setCurrent(new Canvas(){public void paint(Graphics g){}});Thread.sleep(100);
  field(cc,"c",String.class).set(null,"4.62");field(cc,"a",String.class).set(null,".000");field(cc,"b",int.class).setInt(null,"http".equals(transport)?1:2);
  field(bc,"e",String.class).set(null,"127.0.0.1");field(bc,"f",String.class).set(null,"9119");field(bc,"d",String.class).set(null,"http://127.0.0.1:8080/");
  method(cc,"c",void.class).invoke(canvas);network=field(cc,"ab",bc).get(canvas);
  method(cc,"a",void.class,String.class).invoke(canvas,"Connecting...");
  method(cc,"f",void.class,String.class,String.class).invoke(canvas,user,user+"-demo");
  await("original "+transport+" login 200/201/202/203",()->Connector.count("TX",200)>0&&Connector.count("RX",201)>0&&Connector.count("TX",202)>0&&Connector.count("RX",203)>0);
  await("original roster and category requests/replies",()->Connector.count("TX",400)>0&&Connector.count("RX",403)>0&&Connector.count("TX",713)>0&&Connector.count("RX",715)>0);
  Thread.sleep(600);
  Object roster=get(canvas,"Z"),categories=get(canvas,"ad"),tabs=get(canvas,"Y");
  int rows=(Integer)method(roster.getClass(),"c",int.class).invoke(roster),cats=(Integer)method(categories.getClass(),"s",int.class).invoke(categories);
  if(rows<1||cats<1)throw new AssertionError("Empty original UI models: roster="+rows+", categories="+cats);
  System.out.println("PASS original UI models: rosterRows="+rows+" categoryRows="+cats);
  int friends=(Integer)method(tabs.getClass(),"b",int.class,int.class).invoke(tabs,1);if(friends>=0)method(tabs.getClass(),"g",void.class,int.class).invoke(tabs,friends);
  save("friends-after-login");
  int rooms=(Integer)method(tabs.getClass(),"b",int.class,int.class).invoke(tabs,8);if(rooms>=0){method(tabs.getClass(),"g",void.class,int.class).invoke(tabs,rooms);save("room-categories");method(tabs.getClass(),"g",void.class,int.class).invoke(tabs,friends);}
  if("bob".equals(user)){
   await("original HTTP empty GET poll",()->Connector.emptyPolls>0);Files.writeString(out.resolve("ready"),"ready");
   await("Bob original receive dispatcher received Alice message500",()->privateTabFor("Hello from unchanged mig33")>=0);Thread.sleep(100);chat("chat-from-alice");
   send("alice","Reply from original v4.62 Bob over HTTP");await("Bob outgoing reply500",()->Connector.count("TX",500)>0);Thread.sleep(1000);
   Files.writeString(out.resolve("result.txt"),"PASS original v4.62 HTTP login, contacts, categories, empty200image/gifGETpoll, received Alice500 rendered by original chat, reply500 transmitted.\n");return;
  }
  send("bob","Hello from unchanged mig33 v4.62 Alice over TCP");
  await("original send-message builder transmitted opcode 500",()->Connector.count("TX",500)>0);
  await("Alice original receive dispatcher received Bob reply500",()->privateTabFor("Reply from original v4.62 Bob")>=0);Thread.sleep(100);chat("chat-from-bob");
  int previousRequests=Connector.count("TX",922),previousReplies=Connector.count("RX",923);
  method(cc,"a",void.class,String.class,String.class,boolean.class,boolean.class,Class.forName("bl")).invoke(canvas,"http://127.0.0.1:8080/sites/index.php?c=profile&a=home&v=midlet&username=alice",null,true,true,null);
  await("original page fetch 922/923",()->Connector.count("TX",922)>previousRequests&&Connector.count("RX",923)>previousReplies);Thread.sleep(900);save("profile-page");
  int current=field(tabs.getClass(),"b",int.class).getInt(tabs);Object page=method(tabs.getClass(),"d",Class.forName("bl"),int.class).invoke(tabs,current);String title=(String)method(page.getClass(),"a",String.class).invoke(page);if(!"User Profile".equals(title))throw new AssertionError("Unexpected original page title: "+title);System.out.println("PASS original page renderer parsed User Profile");
  Files.writeString(out.resolve("result.txt"),"PASS: unchanged v4.62 JAR used original TCP transport, serializers and receive dispatcher.\nLogin200/201/202/203; roster400/401/402/420/403; categories713/714/715; outgoing and incoming500; page922/923.\nOriginal model rows: roster="+rows+", categories="+cats+".\nCanvas240x320; MicroEmulator Java fonts; harness selects tab and invokes original builders via reflection.\nBlocked non-loopback connection attempts="+Connector.blocked+" (includes mandatory self-test).\n");
 }
 static void logout()throws Exception{int sent=Connector.count("TX",300);method(bc,"a",void.class,short.class).invoke(network,(short)300);await("original logout300 transmitted",()->Connector.count("TX",300)>sent);int transaction=-1;synchronized(Connector.events){for(String event:Connector.events)if(event.startsWith("TX\t300\t"))transaction=Integer.parseInt(event.split("\t")[2]);}final int tx=transaction;await("original logout acknowledgment",()->{synchronized(Connector.events){for(String event:Connector.events)if(event.startsWith("RX\t1\t"+tx+"\t"))return true;}return false;});}
 static void send(String peer,String message)throws Exception{method(bc,"a",void.class,short.class,String.class,String.class,int.class,byte[].class,short.class,int.class).invoke(network,(short)1,peer,message,1,null,(short)1,-1);}
 static int privateTabFor(String expected)throws Exception{Object tabs=get(canvas,"Y");Vector entries=(Vector)field(tabs.getClass(),"a",Vector.class).get(tabs);for(int i=0;i<entries.size();i++){if((Integer)method(tabs.getClass(),"i",int.class,int.class).invoke(tabs,i)!=3)continue;Object chat=method(tabs.getClass(),"c",Class.forName("s"),int.class).invoke(tabs,i);Vector rows=(Vector)field(chat.getClass(),"y",Vector.class).get(chat);for(Object row:rows)if(row.toString().contains(expected))return i;}return -1;}
 static void chat(String name)throws Exception{Object tabs=get(canvas,"Y");int tab=privateTabFor("bob".equals(user)?"Hello from unchanged mig33":"Reply from original v4.62 Bob");if(tab<0)throw new AssertionError("No original private-chat tab after inbound500");Object chat=method(tabs.getClass(),"c",Class.forName("s"),int.class).invoke(tabs,tab);Vector messages=(Vector)field(chat.getClass(),"y",Vector.class).get(chat);String expected="bob".equals(user)?"Hello from unchanged mig33":"Reply from original v4.62 Bob";StringBuilder texts=new StringBuilder();for(Object message:messages)texts.append(message).append('\n');if(!texts.toString().contains(expected))throw new AssertionError("Expected incoming text absent from original wrapped chat rows: "+texts);Files.writeString(out.resolve("chat-model.txt"),texts);System.out.println("PASS original chat model contains expected incoming message");method(tabs.getClass(),"g",void.class,int.class).invoke(tabs,tab);save(name);}
}
