import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.*;

/** Original retained-tab reconnect and failure presentation, isolated backend. */
public class V4ReconnectIntegration extends V4PagesIntegration {
 static final Path reconnectRoot=Paths.get("data/applets/integration/reconnect-out");
 static final String localBase="http://127.0.0.1:"+Connector.HTTP_PORT;
 static String testRoom;
 static void peer(String name,String marker)throws Exception{until(name+" "+marker,()->{if(Files.exists(reconnectRoot.resolve(name).resolve("failed")))throw new AssertionError(name+" failed; see its runtime.log");return Files.exists(reconnectRoot.resolve(name).resolve(marker));});}
 static String openLocal(String path,String capture)throws Exception{return open(localBase+path,capture);}
 static String postLocal(String button,String capture)throws Exception{
  until("native form "+button,()->submitFocus(button)>=0);field(page().getClass(),"r",int.class).setInt(page(),submitFocus(button));String[] form=(String[])method(browser().getClass(),"f",String[].class).invoke(browser());
  if(form==null||!form[1].startsWith(localBase+"/sites/")||form[1].contains("&amp;"))throw new AssertionError("Unsafe native action "+Arrays.toString(form));
  int tx=request(922,923,()->firePage());Files.writeString(out.resolve(capture+"-submitted.txt"),form[1]+"\n"+form[2]+"\n");return acceptPage(tx,capture);
 }
 static void stableRoom(String capture)throws Exception{selectRoom(testRoom);until("original room tab paint ready",()->!field(tabs().getClass(),"c",boolean.class).getBoolean(tabs()));Thread.sleep(150);save(capture);Files.writeString(out.resolve(capture+"-model.txt"),roomRows(testRoom)+"\nready="+field(roomChat(testRoom).getClass(),"d",boolean.class).getBoolean(roomChat(testRoom))+"\n");}
 static void disconnect(String marker)throws Exception{
  clearDialogs();selectRoom(testRoom);field(bc,"j",long.class).setLong(network,120000L);method(cc,"c",void.class,boolean.class).invoke(canvas,true);
  until("original disconnect retained room and clears readiness",()->!field(bc,"c",boolean.class).getBoolean(null)&&roomTab(testRoom)>=0&&!field(roomChat(testRoom).getClass(),"d",boolean.class).getBoolean(roomChat(testRoom)));
  if(!field(cc,"aC",boolean.class).getBoolean(canvas))throw new AssertionError("Original reconnect flag absent");stableRoom(marker);mark(marker);
 }
 static void reconnect(String marker,boolean success,String expected)throws Exception{
  int oldLogin=Connector.count("RX",203),oldJoins=Connector.count("TX",703);
  // Preserve the test's user-selected transport after the original disconnect
  // reloads the handset's saved connectivity preference.
  field(cc,"b",int.class).setInt(null,transport.equals("http")?1:2);method(bc,"e",void.class).invoke(network);
  until("fresh203 on original reconnect",()->Connector.count("RX",203)>oldLogin);
  until("retained tab automatically sent703",()->Connector.count("TX",703)>oldJoins);int tx=transaction("TX",703);
  until("retained-tab rejoin response",()->received(success?1:0,tx));
  if(success){until("original room readiness restored",()->field(roomChat(testRoom).getClass(),"d",boolean.class).getBoolean(roomChat(testRoom)));sendRoom(testRoom,"Accepted after "+marker,1);}else{
   String error=responseText(0,tx,(short)2);if(!error.contains(expected))throw new AssertionError("Wrong rejoin failure "+error);
   until("original rejoin failure displayed in retained room",()->roomRows(testRoom).contains(expected));
   if(field(roomChat(testRoom).getClass(),"d",boolean.class).getBoolean(roomChat(testRoom)))throw new AssertionError("Rejected room incorrectly ready");
   sendRoom(testRoom,"DENIED after "+marker,0);Files.writeString(out.resolve(marker+"-error.txt"),error+"\n");
  }
  stableRoom(marker);mark(marker);
 }
 static void settings(String operation,String value,String label)throws Exception{
  openLocal(url("chatroom","setup")+"&manage=1&roomName="+testRoom.replace(" ","%20"),label+"-form");
  String button=operation.equals("ban")?"Ban User":operation.equals("unban")?"Unban User":"Save Capacity (2 to 100)";
  setFormText(button,"body",value);postLocal(button,label);
 }
 static void owner()throws Exception{
  openLocal(url("chatroom","create"),"create-room");setText("name",testRoom);postLocal("Continue","create-name");setText("description","Native reconnect acceptance");postLocal("Create Room","created");join(testRoom);settings("capacity","2","capacity-two");mark("room-ready");
  peer("bob","normal-rejoined");peer("bob","full-disconnected");mark("fill-room");peer("carol","filled-room");mark("full-ready");peer("bob","full-denied");mark("release-filler");peer("carol","left-room");
  settings("ban","bob","ban-bob");mark("ban-ready");peer("bob","ban-denied");settings("unban","bob","unban-bob");mark("unban-ready");peer("bob","unbanned-rejoined");
  request(706,1,()->method(bc,"a",void.class,short.class,String.class,String.class).invoke(network,(short)706,"bob",testRoom));mark("bob-kicked");peer("bob","kick-rejoined");
  roomParticipants(testRoom,"final-participants","alice","bob");mark("finished");peer("bob","logged-out");peer("carol","logged-out");logout();
 }
 static void returning()throws Exception{
  peer("alice","room-ready");join(testRoom);disconnect("normal-disconnected");reconnect("normal-rejoined",true,"");
  disconnect("full-disconnected");peer("alice","full-ready");reconnect("full-denied",false,"full");
  peer("alice","ban-ready");disconnect("ban-disconnected");reconnect("ban-denied",false,"banned");
  peer("alice","unban-ready");disconnect("unban-disconnected");reconnect("unbanned-rejoined",true,"");
  peer("alice","bob-kicked");until("native kick popup queued",()->Connector.count("RX",5)>0&&!((Vector)get(canvas,"aL")).isEmpty());stableRoom("kick-notification");
  until("native kick popup displayed",()->!((Vector)get(canvas,"aI")).isEmpty());Object popup=((Vector)get(canvas,"aI")).lastElement();String popupText=((Vector)field(popup.getClass(),"a",Vector.class).get(popup)).toString();if(!popupText.replaceAll("[,\\s]+"," ").contains("You were kicked from"))throw new AssertionError("Missing kick presentation "+popupText);Files.writeString(out.resolve("kick-popup.txt"),popupText+"\n");
  if(roomTab(testRoom)<0)throw new AssertionError("Kick unexpectedly closed retained tab");clearDialogs();sendRoom(testRoom,"DENIED after kick",0);stableRoom("kick-send-denied");
  disconnect("kick-disconnected");reconnect("kick-rejoined",true,"");peer("alice","finished");logout();mark("logged-out");
 }
 static void filler()throws Exception{peer("alice","fill-room");join(testRoom);mark("filled-room");peer("alice","release-filler");leave(testRoom);mark("left-room");peer("alice","finished");logout();mark("logged-out");}
 public static void main(String[]args){try{user=args[0];transport=args[1];out=reconnectRoot.resolve(user);Connector.out=out;testRoom=Files.readString(reconnectRoot.resolve("room-name.txt")).trim();V4ContactIntegration.start();if(user.equals("alice"))owner();else if(user.equals("bob"))returning();else filler();Files.writeString(out.resolve("result.txt"),"PASS original v4.62 retained-tab reconnect: "+user+" / "+transport+". Normal, full, banned and kicked-room presentation; fresh native203 and automatic703 observed.\n");System.exit(0);}catch(Throwable e){e.printStackTrace();try{mark("failed");if(network!=null&&field(bc,"c",boolean.class).getBoolean(null))logout();}catch(Throwable ignored){}System.exit(1);}}
}
