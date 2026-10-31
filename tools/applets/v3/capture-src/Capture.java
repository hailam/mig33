import java.lang.reflect.*;
import java.util.*;
import java.io.*;
import java.nio.file.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.lcdui.*;
import org.microemu.app.*;
import org.microemu.device.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;
import org.microemu.MIDletBridge;
/** Calls original mig33 bytecode. Reflection supplies offline fixture state only. */
public class Capture {
 static int width=240,height=320,count=0;
 static Class<?> cc,tc;
 static Object canvas;
 static Path out;
 static java.util.List<String> index=new ArrayList<>();
 static Field field(Class<?> c,String n)throws Exception {Field f=c.getDeclaredField(n);f.setAccessible(true);return f;}
 static Object get(Object o,String n)throws Exception{return field(o.getClass(),n).get(o);}
 static void set(Object o,String n,Object v)throws Exception{field(o.getClass(),n).set(o,v);}
 static void staticSet(Class<?> c,String n,Object v)throws Exception{field(c,n).set(null,v);}
 static Object call(Object o,String n,Class<?>[] types,Object...args)throws Exception{return o.getClass().getMethod(n,types).invoke(o,args);}
 static Object form(String title,int id,String value)throws Exception{return tc.getConstructor(String.class,int.class,String.class).newInstance(title,id,value);}
 static void snapshot(String name,String description,Object painter,String method)throws Exception {
  javax.microedition.lcdui.Image im=javax.microedition.lcdui.Image.createImage(width,height);
  painter.getClass().getMethod(method,Graphics.class).invoke(painter,im.getGraphics());
  save(name,description,im);
 }
 static void save(String name,String description,javax.microedition.lcdui.Image im)throws Exception {
  java.awt.Image awt=((J2SEMutableImage)im).getImage();
  BufferedImage bi=new BufferedImage(width,height,BufferedImage.TYPE_INT_RGB);
  java.awt.Graphics g=bi.getGraphics();g.drawImage(awt,0,0,null);g.dispose();
  ImageIO.write(bi,"png",out.resolve(name+".png").toFile());
  index.add(name+"\t"+description);count++;
 }
 static void captureForm(String name,String title,int id,String value,String note)throws Exception {
  Object f=form(title,id,value);
  // Original login route fills remembered credentials and focuses Username when blank.
  if(id==15)call(f,"b",new Class[]{String.class,String.class},"",null);
  snapshot(name,title+"; state "+id+"; "+note,f,"a");
  int rows=(Integer)call(f,"e",new Class[]{});
  if(rows>10){for(int n=0;n<rows;n++)call(f,"b",new Class[]{});snapshot(name+"-end",title+"; last reachable focus/scroll position; "+note,f,"a");}
 }
 static void menu(String name,String member)throws Exception {
  Object m=get(canvas,member);
  paintMenu(name,m,"Original constructor menu; top selection");
  int rows=(Integer)call(m,"d",new Class[]{});
  for(int n=0;n<rows-1;n++)call(m,"e",new Class[]{});
  paintMenu(name+"-end",m,"Original constructor menu; near bottom selection");
 }
 static void paintMenu(String name,Object m,String note)throws Exception {
  javax.microedition.lcdui.Image im=javax.microedition.lcdui.Image.createImage(width,height); Graphics g=im.getGraphics();
  cc.getMethod("paint",Graphics.class).invoke(canvas,g);
  int mw=(Integer)get(m,"d"),mh=(Integer)get(m,"e"),bar=field(cc,"c").getInt(null),body=field(cc,"d").getInt(null);
  g.setClip(0,0,width,height);g.setColor(160,160,160);g.fillRect((width-mw)/2+3,(body-mh)/2+bar+3,mw,mh);
  g.setClip((width-mw)/2,(body-mh)/2+bar,mw,mh);call(m,"a",new Class[]{Graphics.class},g);
  save(name,note,im);
 }
 public static void main(String[] args){try{run(args);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
 public static void run(String[] args)throws Exception {
  width=240; height=320; // Validated profile; narrow profiles need emulator drawRGB clipping repair.
  out=Paths.get("research/applets/v3/artifacts/screenshots/"+width+"x"+height);Files.createDirectories(out);
  System.setProperty("java.awt.headless","true");
  System.setProperty("microedition.platform","Offline MicroEmulator capture");
  // Prove the shim is the class resolved before any application bytecode starts.
  try{javax.microedition.io.Connector.open("http://offline-proof.invalid");throw new AssertionError("Connector was not blocked");}catch(javax.microedition.io.ConnectionNotFoundException expected){}
  if(!javax.microedition.io.Connector.class.getProtectionDomain().getCodeSource().getLocation().equals(Capture.class.getProtectionDomain().getCodeSource().getLocation()))throw new AssertionError("Wrong Connector loaded");
  Headless headless=new Headless(); Common emu=(Common)get(headless,"emulator");
  org.microemu.EmulatorContext ctx=(org.microemu.EmulatorContext)get(headless,"context");
  ArrayList<String> params=new ArrayList<>(Arrays.asList("--rms","memory","--appclassloader","system","com.projectgoth.Roshan.Main"));
  emu.initParams(params,new org.microemu.app.util.DeviceEntry("Default device",null,DeviceImpl.DEFAULT_LOCATION,true,false),J2SEDevice.class);
  J2SEDeviceDisplay dd=(J2SEDeviceDisplay)ctx.getDeviceDisplay();
  dd.setDisplayRectangle(new Rectangle(0,0,width,height));dd.setDisplayPaintable(new Rectangle(0,0,width,height));
  emu.initMIDlet(true);Thread.sleep(1000);
  javax.microedition.midlet.MIDlet midlet=MIDletBridge.getCurrentMIDlet();
  canvas=Display.getDisplay(midlet).getCurrent();cc=Class.forName("com.projectgoth.Roshan.c");tc=Class.forName("t");
  // Detach the original Canvas so emulator repaint events cannot race fixture updates.
  Display.getDisplay(midlet).setCurrent(new Canvas(){public void paint(Graphics g){}});Thread.sleep(100);
  staticSet(cc,"t",false);
  if(get(canvas,"g")==null){Method r=cc.getDeclaredMethod("r");r.setAccessible(true);r.invoke(canvas);}
  staticSet(Class.forName("com.projectgoth.Roshan.Main"),"d","3.08");
  staticSet(cc,"z","+9600000000");staticSet(cc,"ao",true);
  captureForm("01-about","About mig33",1,null,"static");
  captureForm("03-contact-type","Add Contact",3,null,"static");
  captureForm("04-add-mig33-contact","Add mig33 contact",4,null,"blank fields");
  captureForm("08-contact-request-fixture","Contact Request",8,"sample.friend","SYNTHETIC requester");
  captureForm("09-contact-details","Contact Details",9,null,"blank fields");
  captureForm("11-create-room","Create Room",11,null,"blank fields");
  captureForm("13-manual-connection","Connection Type",13,null,"original endpoint defaults");
  captureForm("14-forgot-details-fixture","Forgot Details",14,null,"SYNTHETIC mobile number");
  captureForm("15-login","Login",15,null,"empty RMS defaults; Username focus");
  captureForm("17-make-call","Make Call",17,null,"call-through capability absent");
  staticSet(cc,"ax",true);captureForm("17-make-call-through","Make Call",17,null,"capability flag forced on");staticSet(cc,"ax",false);
  for(String provider:new String[]{"MSN","Yahoo!","Google Talk","AIM"}) captureForm("22-"+provider.replaceAll("[^A-Za-z]","").toLowerCase()+"-setup",provider+" Setup",22,provider,"blank fields");
  captureForm("23-invite-friend","Invite Friend",23,null,"blank fields");
  captureForm("34-search-room","Search Room",34,null,"blank fields");
  captureForm("40-resend-code-fixture","Resend Code",40,null,"SYNTHETIC mobile number");
  captureForm("41-authenticate","Authenticate Account",41,null,"static");
  captureForm("42-connection-type","Connection Type",42,null,"static");
  captureForm("43-welcome-tcp","Welcome to mig33",43,null,"successful TCP state forced; no live connection");
  captureForm("49-first-run","mig33",49,null,"static");
  // Register screen needs server-provided CAPTCHA bytes. Supply an obviously synthetic fixture.
  BufferedImage captcha=new BufferedImage(100,28,BufferedImage.TYPE_INT_RGB);java.awt.Graphics cg=captcha.getGraphics();cg.setColor(java.awt.Color.WHITE);cg.fillRect(0,0,100,28);cg.setColor(java.awt.Color.BLACK);cg.drawString("FIXTURE",8,19);cg.dispose();
  ByteArrayOutputStream cb=new ByteArrayOutputStream();ImageIO.write(captcha,"png",cb);
  Class<?> wc=Class.forName("w"),pc=Class.forName("p");Object packet=wc.getConstructor().newInstance();
  call(packet,"a",new Class[]{pc},pc.getConstructor(short.class,String.class).newInstance((short)1,"+9600000000"));
  call(packet,"a",new Class[]{pc},pc.getConstructor(short.class,byte[].class).newInstance((short)3,cb.toByteArray()));
  staticSet(tc,"j",packet);
  if(width>=240)captureForm("30-registration-fixture","Registration",30,null,"SYNTHETIC CAPTCHA and mobile number");
  else System.out.println("SKIP registration at narrow profile: MicroEmulator 2.0.4 drawRGB clipping limitation");
  staticSet(cc,"al",10);set(canvas,"j",null);set(canvas,"k",null);
  snapshot("10-contacts-empty","Empty contacts state; offline original constructor",canvas,"paint");
  for(String[] pair:new String[][]{{"menu-contact","aE"},{"menu-group","aF"},{"menu-settings","aG"},{"menu-chatrooms","aI"},{"menu-mail","aJ"}})menu(pair[0],pair[1]);
  Object roomPacket=wc.getConstructor().newInstance();
  call(roomPacket,"a",new Class[]{pc},pc.getConstructor(short.class,String.class).newInstance((short)1,"Sample Room"));
  call(roomPacket,"a",new Class[]{pc},pc.getConstructor(short.class,int.class).newInstance((short)3,25));
  call(roomPacket,"a",new Class[]{pc},pc.getConstructor(short.class,int.class).newInstance((short)4,3));
  ((Vector)get(canvas,"o")).add(roomPacket);
  for(String[] m:new String[][]{{"menu-other-ims","k"},{"menu-camera-resolutions","l"},{"menu-chatroom-list-fixture","n"}}){
   call(canvas,m[1],new Class[]{});Object popup=get(canvas,"aL");set(canvas,"bm",0);paintMenu(m[0],popup,"Original offline menu factory");
  }
  set(canvas,"y",85);call(canvas,"j",new Class[]{});Object imMenu=get(canvas,"aL");set(canvas,"bm",0);paintMenu("menu-msn-account",imMenu,"Original IM account menu factory, no saved account");
  // Add explicitly synthetic contact model directly to original roster storage.
  Object roster=get(canvas,"g");Class<?> nc=Class.forName("n");
  Vector entries=(Vector)field(roster.getClass(),"n").get(null);
  Object group=nc.getConstructor(String.class,int.class,boolean.class,int.class).newInstance("Friends",1,false,2);entries.add(group);
  Object friend=nc.getConstructor(String.class,int.class,int.class,int.class).newInstance("sample.friend",1,1,0);set(friend,"e","sample.friend");set(friend,"f",(byte)1);set(friend,"a",101);entries.add(friend);
  Object away=nc.getConstructor(String.class,int.class,int.class,int.class).newInstance("away.friend",1,1,0);set(away,"e","away.friend");set(away,"f",(byte)4);set(away,"a",102);entries.add(away);
  call(roster,"d",new Class[]{});snapshot("10-contacts-fixture","SYNTHETIC group and contacts; original roster renderer",canvas,"paint");
  Object tabs=get(canvas,"f");call(tabs,"a",new Class[]{nc,int.class,int.class},friend,1,1);
  Object chat=call(tabs,"a",new Class[]{int.class},0);
  call(chat,"a",new Class[]{int.class,String.class,String.class,String.class,short.class,int.class,int.class},1,"sample.friend","Hello from a synthetic fixture.",null,(short)1,-1,-1);
  call(chat,"a",new Class[]{int.class,String.class,String.class,String.class,short.class,int.class,int.class},0,"sample.you","This demonstrates the original chat renderer.",null,(short)1,-1,-1);
  staticSet(cc,"al",6);snapshot("06-chat-fixture","SYNTHETIC private chat messages; original tab/chat renderer",canvas,"paint");
  Files.write(out.resolve("index.tsv"),index);
  System.out.println("Captured "+count+" images at "+width+"x"+height+"; Connector blocks="+javax.microedition.io.Connector.blocked+" (includes one mandatory self-test)");
  System.exit(0);
 }
}
