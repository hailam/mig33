import java.lang.reflect.*;
import java.util.*;
import java.nio.file.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.lcdui.*;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;
import org.microemu.MIDletBridge;
/** Offline rendering of the preserved v4.62 bytecode, not the decompiler output. */
public class V4Capture {
 static Class<?> cc,yc;static Object canvas;static final Path out=Paths.get("research/applets/v4/artifacts/screenshots/240x320");
 static java.util.List<String> index=new ArrayList<>();
 static Field field(Class<?> c,String n,Class<?> type)throws Exception{for(Field f:c.getDeclaredFields())if(f.getName().equals(n)&&(type==null||f.getType()==type)){f.setAccessible(true);return f;}throw new NoSuchFieldException(n);}
 static Object get(Object o,String n)throws Exception{return field(o.getClass(),n,null).get(o);}
 static Method method(Class<?> c,String n,Class<?> returns,Class<?>...types)throws Exception{for(Method m:c.getDeclaredMethods())if(m.getName().equals(n)&&m.getReturnType()==returns&&Arrays.equals(m.getParameterTypes(),types)){m.setAccessible(true);return m;}throw new NoSuchMethodException(n);}
 static String label(int id)throws Exception{return (String)method(yc,"e",String.class,int.class).invoke(null,id);}
 static void save(String name,String note,Object painter,String methodName)throws Exception{
  javax.microedition.lcdui.Image im=javax.microedition.lcdui.Image.createImage(240,320);
  method(painter.getClass(),methodName,void.class,Graphics.class).invoke(painter,im.getGraphics());
  BufferedImage bi=new BufferedImage(240,320,BufferedImage.TYPE_INT_RGB);java.awt.Graphics g=bi.getGraphics();g.drawImage(((J2SEMutableImage)im).getImage(),0,0,null);g.dispose();
  ImageIO.write(bi,"png",out.resolve(name+".png").toFile());index.add(name+"\t"+note);
 }
 static void form(int state,int labelId,String name,String text)throws Exception{
  String title=labelId>=0?label(labelId):"mig33";
  try{
   Object f;
   if(state==60){
    BufferedImage fixture=new BufferedImage(100,28,BufferedImage.TYPE_INT_RGB);java.awt.Graphics fg=fixture.getGraphics();fg.setColor(java.awt.Color.WHITE);fg.fillRect(0,0,100,28);fg.setColor(java.awt.Color.BLACK);fg.drawString("FIXTURE",8,19);fg.dispose();
    java.io.ByteArrayOutputStream bytes=new java.io.ByteArrayOutputStream();ImageIO.write(fixture,"png",bytes);byte[] png=bytes.toByteArray();
    javax.microedition.lcdui.Image fi=javax.microedition.lcdui.Image.createImage(png,0,png.length);
    f=yc.getConstructor(String.class,int.class,String.class,Vector.class,javax.microedition.lcdui.Image.class,String.class,String.class).newInstance(title,state,null,null,fi,null,"Enter the sample security code.");
   }else f=yc.getConstructor(String.class,int.class,String.class,Vector.class).newInstance(title,state,text,new Vector());
   save(name,title+"; original form constructor state "+state+(state==1?"; SYNTHETIC IM provider":state==60?"; SYNTHETIC CAPTCHA image and prompt":state==14||state==40?"; SYNTHETIC mobile number":text!=null?"; SYNTHETIC supplied text":"; empty offline defaults"),f,"a");
   int rows=(Integer)method(yc,"e",int.class).invoke(f);
   if(rows>10){for(int n=0;n<rows;n++)method(yc,"o",void.class).invoke(f);save(name+"-end",title+"; final focus/scroll position; same fixture values",f,"a");}
  }catch(Throwable e){System.out.println("SKIP "+state+" "+e+" "+e.getCause());}
 }
 public static void main(String[] args){try{run();System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
 static void run()throws Exception{
  Files.createDirectories(out);System.setProperty("java.awt.headless","true");System.setProperty("microedition.platform","Offline MicroEmulator capture");
  try{javax.microedition.io.Connector.open("http://offline-proof.invalid");throw new AssertionError();}catch(javax.microedition.io.ConnectionNotFoundException expected){}
  if(!javax.microedition.io.Connector.class.getProtectionDomain().getCodeSource().getLocation().toString().contains("capture-classes"))throw new AssertionError("Wrong Connector");
  Headless headless=new Headless();Common emu=(Common)get(headless,"emulator");org.microemu.EmulatorContext ctx=(org.microemu.EmulatorContext)get(headless,"context");
  emu.initParams(new ArrayList<String>(Arrays.asList("--rms","memory","--appclassloader","system","main.Main")),new org.microemu.app.util.DeviceEntry("Default device",null,DeviceImpl.DEFAULT_LOCATION,true,false),J2SEDevice.class);
  J2SEDeviceDisplay dd=(J2SEDeviceDisplay)ctx.getDeviceDisplay();dd.setDisplayRectangle(new Rectangle(0,0,240,320));dd.setDisplayPaintable(new Rectangle(0,0,240,320));
  emu.initMIDlet(true);Thread.sleep(1600);
  javax.microedition.midlet.MIDlet midlet=MIDletBridge.getCurrentMIDlet();canvas=Display.getDisplay(midlet).getCurrent();
  cc=Class.forName("aj");yc=Class.forName("y");System.out.println("Canvas="+canvas+" dimensions="+field(cc,"e",int.class).get(null)+"x"+field(cc,"f",int.class).get(null));
  Display.getDisplay(midlet).setCurrent(new Canvas(){public void paint(Graphics g){}});Thread.sleep(100);
  field(cc,"c",String.class).set(null,"4.62");field(cc,"a",String.class).set(null,".000");field(cc,"o",String.class).set(null,"+9600000000");
  // Registry entries normally supplied by the server. This explicit provider is a fixture.
  Class<?> bjc=Class.forName("bj"); Object provider=bjc.getConstructor().newInstance();
  field(bjc,"a",String.class).set(provider,"MSN (fixture)");field(bjc,"f",byte.class).setByte(provider,(byte)2);field(bjc,"d",byte.class).setByte(provider,(byte)1);
  ((Hashtable)field(cc,"aS",Hashtable.class).get(null)).put(Integer.valueOf(1),provider);
  field(cc,"bh",int.class).setInt(null,Integer.MAX_VALUE);
  save("startup","Original offline startup state; all network refused",canvas,"paint");
  form(1,372,"01-add-im-contact-fixture",null);form(9,87,"09-contact-details",null);form(13,86,"13-manual-connection",null);
  form(14,126,"14-forgot-details-fixture",null);form(15,183,"15-login",null);form(23,158,"23-invite-friend",null);
  form(30,248,"30-registration-no-captcha",null);form(36,152,"36-information-fixture","Synthetic offline information.\nThis is sample content.");
  form(40,262,"40-resend-code-fixture",null);form(41,8,"41-authenticate",null);form(43,340,"43-welcome",null);form(49,193,"49-first-run",null);
  form(52,15,"52-add-phone-contact",null);form(54,86,"54-connection-type",null);form(57,11,"57-add-mig33-contact",null);form(62,248,"62-registration-username",null);form(60,386,"60-captcha-fixture",null);
  method(cc,"c",void.class).invoke(canvas); // Original login factory used as menu underlay.
  field(cc,"aF",yc).set(canvas,null);field(cc,"aG",yc).set(canvas,null);
  ((Vector)get(canvas,"aI")).clear();
  for(String factory:new String[]{"G","J","N","Q","S","X","I","Y","P","W","H","ab","T","M","O","K","L","R"}) {
   try {
    Object popup=method(cc,factory,yc).invoke(canvas);
    if(popup==null)continue;
    Vector stack=(Vector)get(canvas,"aH");stack.clear();stack.add(popup);
    field(cc,"bh",int.class).setInt(null,Integer.MAX_VALUE);
    save("menu-"+factory,"Original menu factory aj."+factory+"(); offline flags; login underlay",canvas,"paint");stack.clear();
   }catch(Throwable e){System.out.println("SKIP menu "+factory+" "+e+" "+e.getCause());}
  }
  Files.write(out.resolve("index.tsv"),index);System.out.println("Captured "+index.size()+" v4 images; Connector blocks="+javax.microedition.io.Connector.blocked+" (one mandatory self-test)");
 }
}
