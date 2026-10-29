import java.util.*;
import java.nio.file.*;
import javax.microedition.io.Connector;

/** Public rooms through unchanged original v4.62 builders, transport and UI. */
public class V4RoomIntegration extends V4GroupIntegration {
 static final Path roomRoot=Paths.get("data/applets/integration/room-out");
 static void peerMark(String peer,String name)throws Exception{until(peer+" reached "+name,()->Files.exists(roomRoot.resolve(peer).resolve(name)));}
 static int roomTab(String room)throws Exception{Object ts=tabs();Vector entries=(Vector)field(ts.getClass(),"a",Vector.class).get(ts);for(int i=0;i<entries.size();i++){if((Integer)method(ts.getClass(),"i",int.class,int.class).invoke(ts,i)==5){Object chat=method(ts.getClass(),"c",Class.forName("s"),int.class).invoke(ts,i);if(room.equals(field(chat.getClass(),"b",String.class).get(chat)))return i;}}return -1;}
 static Object roomChat(String room)throws Exception{int tab=roomTab(room);return tab<0?null:method(tabs().getClass(),"c",Class.forName("s"),int.class).invoke(tabs(),tab);}
 static void selectRoom(String room)throws Exception{int tab=roomTab(room);if(tab<0)throw new AssertionError("No original room tab: "+room);method(tabs().getClass(),"g",void.class,int.class).invoke(tabs(),tab);}
 static String roomRows(String room)throws Exception{Object chat=roomChat(room);if(chat==null)return "";Vector rows=(Vector)field(chat.getClass(),"y",Vector.class).get(chat);StringBuilder b=new StringBuilder();synchronized(rows){for(Object row:rows)b.append(row).append('\n');}return b.toString();}
 static boolean wireHasRoomText(String room,String phrase)throws Exception{
  List<String> files=new ArrayList<>();synchronized(Connector.events){for(String e:Connector.events){String[] p=e.split("\t");if(p.length>=5&&p[0].equals("RX")&&p[1].equals("500"))files.add(p[4]);}}
  Class<?> ay=Class.forName("ay");for(String file:files){Object[] packets=(Object[])method(ay,"a",java.lang.reflect.Array.newInstance(ay,0).getClass(),byte[].class).invoke(null,Files.readAllBytes(out.resolve(file)));for(Object packet:packets){Byte kind=(Byte)method(ay,"b",Byte.class,short.class).invoke(packet,(short)3);String destination=(String)method(ay,"f",String.class,short.class).invoke(packet,(short)4);String body=(String)method(ay,"f",String.class,short.class).invoke(packet,(short)8);if(kind!=null&&kind==3&&room.equals(destination)&&body!=null&&body.contains(phrase))return true;}}
  return false;
 }
 static Map<String,String> catalogRows()throws Exception{Object model=get(canvas,"ad");Vector rows=(Vector)field(model.getClass(),"f",Vector.class).get(model);Map<String,String> rooms=new TreeMap<>();synchronized(rows){for(Object row:rows)if(field(row.getClass(),"c",int.class).getInt(row)==2)rooms.put((String)field(row.getClass(),"c",String.class).get(row),(String)field(row.getClass(),"a",String.class).get(row));}return rooms;}
 static void catalog(String capture,int lobby,int gaming)throws Exception{
  Object model=get(canvas,"ad");Object category=method(model.getClass(),"b",model.getClass(),int.class).invoke(model,1);if(category==null)throw new AssertionError("Missing original room category1");method(model.getClass(),"a",void.class,boolean.class,model.getClass()).invoke(model,false,category);
  int before=Connector.count("RX",717);method(cc,"q",void.class).invoke(canvas);until("original category716/701/717 completed",()->Connector.count("RX",717)>before);
  until("original room catalog contains three seeded rooms",()->catalogRows().keySet().equals(new TreeSet<>(Arrays.asList("Gaming","Help","Lobby"))));
  Map<String,String> rows=catalogRows();if(lobby>=0&&!rows.get("Lobby").equals("("+lobby+"/50)"))throw new AssertionError("Wrong Lobby occupancy "+rows);if(gaming>=0&&!rows.get("Gaming").equals("("+gaming+"/50)"))throw new AssertionError("Wrong Gaming occupancy "+rows);
  int tab=(Integer)method(tabs().getClass(),"b",int.class,int.class).invoke(tabs(),8);if(tab<0)throw new AssertionError("No original room catalog tab");method(tabs().getClass(),"g",void.class,int.class).invoke(tabs(),tab);save(capture);Files.writeString(out.resolve(capture+"-model.txt"),rows.toString()+"\n");System.out.println("PASS original room catalog "+rows);
 }
 static void join(String room)throws Exception{
  int tx=request(703,701,()->method(cc,"b",void.class,String.class,boolean.class).invoke(canvas,room,false));
  until("join701 followed by correlated acknowledgment",()->received(1,tx));until("original joined room model ready",()->roomChat(room)!=null&&field(roomChat(room).getClass(),"d",boolean.class).getBoolean(roomChat(room)));
  until("original pending703 completed",()->!(Boolean)method(bc,"b",boolean.class,short.class).invoke(network,(short)703));selectRoom(room);save("joined-"+room.toLowerCase());
 }
 static void roomParticipants(String room,String capture,String... expected)throws Exception{
  selectRoom(room);method(cc,"k",void.class).invoke(canvas);request(707,708,()->method(bc,"a",void.class,short.class,String.class).invoke(network,(short)707,room));
  until("original room participant popup constructed",()->method(cc,"i",Class.forName("y")).invoke(canvas)!=null);Object popup=method(cc,"i",Class.forName("y")).invoke(canvas);int count=(Integer)method(popup.getClass(),"c",int.class).invoke(popup);TreeSet<String> names=new TreeSet<>();for(int i=0;i<count;i++)names.add((String)method(popup.getClass(),"a",String.class,int.class).invoke(popup,i));
  if(count!=expected.length||!names.equals(new TreeSet<>(Arrays.asList(expected))))throw new AssertionError("Room participants mismatch: "+room+" "+names);save(capture);Files.writeString(out.resolve(capture+"-model.txt"),String.join("\n",names)+"\n");System.out.println("PASS original "+room+" participant popup "+names);method(cc,"k",void.class).invoke(canvas);
 }
 static void sendRoom(String room,String text,int response)throws Exception{request(500,response,()->method(bc,"a",void.class,short.class,String.class,String.class,int.class,byte[].class,short.class,int.class).invoke(network,(short)1,room,text,3,null,(short)1,-1));}
 static void expectRoom(String room,String phrase,String capture)throws Exception{until("original "+room+" rows contain "+phrase,()->roomRows(room).contains(phrase));if(!wireHasRoomText(room,phrase))throw new AssertionError("UI text lacks matching original incoming room packet");selectRoom(room);save(capture);Files.writeString(out.resolve(capture+"-model.txt"),roomRows(room));}
 static void leave(String room)throws Exception{
  selectRoom(room);int tab=roomTab(room);request(704,1,()->{((Vector)field(bc,"o",Vector.class).get(network)).add(room);method(bc,"a",void.class,short.class,String.class).invoke(network,(short)704,room);method(tabs().getClass(),"f",boolean.class,int.class).invoke(tabs(),tab);method(cc,"k",void.class).invoke(canvas);});
  until("original leave acknowledgment clears suppression",()->!((Vector)field(bc,"o",Vector.class).get(network)).contains(room));if(roomTab(room)>=0)throw new AssertionError("Original room tab was not closed on leave");save("after-leave-"+room.toLowerCase());
 }
 static void alice()throws Exception{
  peerMark("bob","browsed");peerMark("carol","browsed");join("Lobby");mark("joined-lobby");peerMark("bob","joined-lobby");peerMark("carol","joined-gaming");roomParticipants("Lobby","participants-two","alice","bob");catalog("rooms-two-lobby-one-gaming",2,1);
  sendRoom("Lobby","Lobby phase1 hello",1);peerMark("bob","phase1-received");mark("phase1-delivered");peerMark("carol","outsider-denied");mark("may-join-lobby");peerMark("carol","joined-lobby");roomParticipants("Lobby","participants-three","alice","bob","carol");catalog("rooms-three-lobby-one-gaming",3,1);
  sendRoom("Lobby","Lobby phase2 hello",1);expectRoom("Lobby","Bob room reply","room-bob-reply");expectRoom("Lobby","Carol room reply","room-carol-reply");peerMark("bob","carol-reply-received");peerMark("carol","bob-reply-received");mark("may-leave-lobby");peerMark("carol","postleave-denied");roomParticipants("Lobby","participants-after-leave","alice","bob");catalog("rooms-after-carol-leaves",2,1);
  sendRoom("Lobby","Lobby after Carol left",1);peerMark("bob","afterleave-received");mark("final-sent");peerMark("carol","no-postleave-delivery");mark("finished");
 }
 static void bob()throws Exception{
  join("Lobby");mark("joined-lobby");expectRoom("Lobby","Lobby phase1 hello","room-phase1");mark("phase1-received");expectRoom("Lobby","Lobby phase2 hello","room-phase2");sendRoom("Lobby","Bob room reply",1);expectRoom("Lobby","Carol room reply","room-carol-reply");mark("carol-reply-received");expectRoom("Lobby","Lobby after Carol left","room-after-carol-leaves");mark("afterleave-received");peerMark("alice","finished");
 }
 static void carol()throws Exception{
  join("Gaming");roomParticipants("Gaming","gaming-only-carol","carol");mark("joined-gaming");peerMark("alice","phase1-delivered");roomParticipants("Gaming","gaming-isolated","carol");if(wireHasRoomText("Lobby","Lobby phase1 hello"))throw new AssertionError("Gaming member received Lobby text without membership");System.out.println("PASS room isolation in captured original incoming packets");
  sendRoom("Lobby","DENIED outsider room send",0);Thread.sleep(200);if(roomTab("Lobby")>=0){selectRoom("Lobby");save("outsider-denied");}mark("outsider-denied");peerMark("alice","may-join-lobby");join("Lobby");mark("joined-lobby");expectRoom("Lobby","Lobby phase2 hello","room-phase2");expectRoom("Lobby","Bob room reply","room-bob-reply");mark("bob-reply-received");sendRoom("Lobby","Carol room reply",1);peerMark("alice","may-leave-lobby");leave("Lobby");sendRoom("Lobby","DENIED room post-leave",0);Thread.sleep(200);if(roomTab("Lobby")>=0){selectRoom("Lobby");save("postleave-denied");}mark("postleave-denied");peerMark("alice","final-sent");roomParticipants("Gaming","gaming-membership-retained","carol");
  if(wireHasRoomText("Lobby","Lobby after Carol left"))throw new AssertionError("Former Lobby member received final room text");System.out.println("PASS no post-leave Lobby packet; Gaming membership remains active");mark("no-postleave-delivery");peerMark("alice","finished");
 }
 public static void main(String[] args){try{user=args[0];transport=args[1];out=roomRoot.resolve(user);Connector.out=out;init(false);catalog("room-list-initial",-1,-1);mark("browsed");if(user.equals("alice"))alice();else if(user.equals("bob"))bob();else carol();logout();Files.writeString(out.resolve("result.txt"),"PASS original v4.62 public room integration: "+user+" over "+transport+". Original catalog, joined room model, participant popups, room text and membership enforcement.\n");System.out.println("PASS "+user+" public rooms complete");System.exit(0);}catch(Throwable e){e.printStackTrace();System.exit(1);}}
}
