import java.nio.file.*;
import java.util.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.io.Connector;

/** Three unchanged clients: two Alice sessions and Bob. */
public class V4MirrorIntegration extends V4SlashIntegration {
    static final Path mirrorRoot=Paths.get("data/applets/integration/mirror-out");
    static final String direct="Mirror proof private01", grouped="Mirror proof group01";
    static String role;

    static void peerMirror(String who,String marker)throws Exception {
        until(who+" reached "+marker,()->Files.exists(mirrorRoot.resolve(who).resolve(marker)));
    }
    static int occurrences(String text,String needle){int count=0,index=0;while((index=text.indexOf(needle,index))>=0){count++;index+=needle.length();}return count;}
    static void capture(String name)throws Exception {
        long deadline=System.currentTimeMillis()+15000,previous=0;boolean populatedBefore=false;
        while(System.currentTimeMillis()<deadline){
            V4Integration.save(name);
            BufferedImage image=ImageIO.read(out.resolve(name+".png").toFile());
            Map<Integer,Integer> colors=new HashMap<>();long hash=1125899906842597L;
            for(int y=24;y<306;y++)for(int x=4;x<232;x++){int pixel=image.getRGB(x,y)&0xffffff;colors.put(pixel,colors.getOrDefault(pixel,0)+1);hash=hash*31+pixel;}
            double dominant=(double)Collections.max(colors.values())/(228*282);
            boolean populated=colors.size()>=3&&dominant<0.995&&!field(tabs().getClass(),"c",boolean.class).getBoolean(tabs());
            if(populated&&populatedBefore&&hash==previous){Files.writeString(out.resolve("capture-metrics.tsv"),name+"\t"+colors.size()+"\t"+dominant+"\n",StandardOpenOption.CREATE,StandardOpenOption.APPEND);return;}
            populatedBefore=populated;previous=hash;Thread.sleep(100);
        }
        throw new AssertionError("Unpainted/unstable capture "+name);
    }
    static void verifyChat(int kind,String target,String phrase,String capture)throws Exception {
        until("original chat contains "+phrase,()->visibleRows(kind,target).contains(phrase));
        String rows=visibleRows(kind,target);
        if(occurrences(rows,phrase)!=1)throw new AssertionError("Duplicate message in original model: "+rows);
        selectConversation(kind,target);capture(capture);Files.writeString(out.resolve(capture+"-model.txt"),rows+"\n");
    }
    static void checkPacket(int kind,String body,String sender,short content)throws Exception {
        List<Object> found=incoming(kind,body);if(found.size()!=1)throw new AssertionError("Expected exactly one incoming packet for "+body+": "+found.size());
        Object p=found.get(0);Short value=(Short)method(p.getClass(),"c",Short.class,short.class).invoke(p,(short)6);
        if(!sender.equals(textField(p,2))||value==null||value!=content)throw new AssertionError("Wrong original routing/style packet");
    }
    static void waitGroup()throws Exception {
        until("shared group ID",()->Files.exists(mirrorRoot.resolve("group-id.txt")));
        group=Files.readString(mirrorRoot.resolve("group-id.txt"));until("original group tab created",()->groupTab()>=0);
    }
    static void origin()throws Exception {
        peerMirror("mirror","ready");peerMirror("bob","ready");
        nativeSend(1,"bob",direct,1,"private-send");
        verifyChat(1,"bob",direct,"private-origin");
        peerMirror("mirror","private-verified");peerMirror("bob","private-verified");
        Thread.sleep(600);
        if(occurrences(visibleRows(1,"bob"),direct)!=1||!incoming(1,direct).isEmpty()||!incoming(1,"alice: "+direct).isEmpty())throw new AssertionError("Origin local echo duplicated by server");
        mark("private-verified");
        int tx=request(751,750,()->method(bc,"a",void.class,String.class,String.class,String.class,byte.class).invoke(network,"bob","carol",null,(byte)1));
        group=responseText(750,tx,(short)1);Files.writeString(mirrorRoot.resolve("group-id.txt"),group);
        until("origin group converted",()->groupTab()>=0);
        peerMirror("mirror","group-ready");peerMirror("bob","group-ready");
        nativeSend(2,group,grouped,1,"group-send");verifyChat(2,group,grouped,"group-origin");
        peerMirror("mirror","group-verified");peerMirror("bob","group-verified");
        if(!incoming(2,grouped).isEmpty()||occurrences(visibleRows(2,group),grouped)!=1)throw new AssertionError("Origin group echo duplicated");
        mark("group-verified");peerMirror("bob","contacts-verified");mark("finished");
    }
    static void mirror()throws Exception {
        verifyChat(1,"bob","alice: "+direct,"private-own-mirror");checkPacket(1,"alice: "+direct,"bob",(short)6);
        if(privateIndex("alice")>=0)throw new AssertionError("Own mirror opened a self-addressed tab");mark("private-verified");
        waitGroup();mark("group-ready");verifyChat(2,group,grouped,"group-own-mirror");checkPacket(2,grouped,"alice",(short)1);mark("group-verified");peerMirror("origin","finished");
    }
    static Object displayedContact(String display)throws Exception {
        Object roster=V4ContactIntegration.roster();int count=(Integer)method(roster.getClass(),"c",int.class).invoke(roster);
        for(int i=0;i<count;i++){Object row=method(roster.getClass(),"e",Class.forName("t"),int.class).invoke(roster,i);if(display.equals(field(row.getClass(),"c",String.class).get(row)))return row;}return null;
    }
    static void addExternal(String display,String phone,String external,byte service)throws Exception {
        request(405,402,()->method(bc,"a",void.class,String.class,String.class,String.class,String.class,String.class,String.class,String.class,String.class,int.class,boolean.class,String.class,String.class,byte.class)
            .invoke(network,display,phone,null,null,null,null,null,null,1,false,null,external,service));
        until("native contact row "+display,()->displayedContact(display)!=null);
    }
    static void contacts()throws Exception {
        V4ContactIntegration.friends("contacts-before");
        addExternal("Phone friend","+9607654321",null,(byte)-1);
        Object phone=displayedContact("Phone friend");if(field(phone.getClass(),"e",String.class).get(phone)!=null||!"+9607654321".equals(field(phone.getClass(),"d",String.class).get(phone)))throw new AssertionError("Phone402 selected wrong branch");
        addExternal("External friend",null,"friend@example.test",(byte)2);
        Object im=displayedContact("External friend");Hashtable identities=(Hashtable)field(im.getClass(),"f",Hashtable.class).get(im),presences=(Hashtable)field(im.getClass(),"g",Hashtable.class).get(im);
        if(field(im.getClass(),"e",String.class).get(im)!=null||!"friend@example.test".equals(identities.get("2"))||!Integer.valueOf(99).equals(presences.get("2")))throw new AssertionError("External402 identities/presence mismatch");
        if(field(im.getClass(),"r",int.class).getInt(im)!=2)throw new AssertionError("External descriptor must select original dynamic-provider icons");
        V4ContactIntegration.friends("phone-external-contacts");capture("phone-external-contacts");
        Object roster=V4ContactIntegration.roster();javax.microedition.lcdui.Image surface=javax.microedition.lcdui.Image.createImage(240,320);
        // Call the original list renderer directly as well: aj.paint catches
        // an icon error and can leave an apparently populated partial list.
        method(roster.getClass(),"a",int.class,javax.microedition.lcdui.Graphics.class,boolean.class).invoke(roster,surface.getGraphics(),false);
        if(!"External friend".equals(field(im.getClass(),"t",String.class).get(im))||!"Phone friend".equals(field(phone.getClass(),"t",String.class).get(phone)))throw new AssertionError("Original renderer did not reach both new contact labels");
        command("bw");selectMenu(label(188),"local-directory-menu-entry");command("bD");
        Object menu=method(cc,"i",Class.forName("y")).invoke(canvas);int count=(Integer)method(menu.getClass(),"c",int.class).invoke(menu);StringBuilder labels=new StringBuilder();
        for(int i=0;i<count;i++)labels.append(method(menu.getClass(),"a",String.class,int.class).invoke(menu,i)).append('\n');
        if(!labels.toString().contains("MSN saved contacts (local)"))throw new AssertionError("Visible local-directory label missing");
        capture("local-directory-menu");Files.writeString(out.resolve("local-directory-menu-model.txt"),labels);clearDialogs();
        int connect=request(206,1,()->method(bc,"a",void.class,short.class,int.class,int.class,boolean.class).invoke(network,(short)206,2,1,false));
        String receipt=responseText(1,connect,(short)1);
        if(!receipt.contains("Local saved contacts are available")||!receipt.contains("external IM provider is disconnected")||!receipt.contains("no external sign-in occurred"))throw new AssertionError("Misleading local-directory acknowledgment: "+receipt);
        clearDialogs();
        request(302,1,()->method(bc,"a",void.class,short.class,int.class,int.class,boolean.class).invoke(network,(short)302,2,-1,false));
        Class<?> ay=Class.forName("ay"),bj=Class.forName("bj");
        Object settings=ay.getConstructor(short.class).newInstance((short)906),service=bj.getConstructor(short.class,byte.class).newInstance((short)30,(byte)2);
        method(ay,"a",void.class,bj).invoke(settings,service);field(cc,"aD",ay).set(canvas,settings);
        // Match the selected provider/removal-confirmation context used by
        // the original UI; ACK907 consults these fields before closing it.
        field(cc,"ao",int.class).setInt(canvas,2);field(cc,"aN",int.class).setInt(canvas,3);
        request(907,1,()->method(bc,"b",void.class,byte.class).invoke(network,(byte)2));
        until("local directory restored after settings removal",()->{Object descriptor=method(cc,"h",bj,int.class).invoke(null,2);return field(bj,"f",byte.class).getByte(descriptor)==2;});
        if(Connector.count("RX",207)!=0)throw new AssertionError("Local directory falsely received external provider connection state");
        clearDialogs();V4ContactIntegration.friends("contacts-after-im-settings");capture("contacts-after-im-settings");
        method(roster.getClass(),"a",int.class,javax.microedition.lcdui.Graphics.class,boolean.class).invoke(roster,surface.getGraphics(),false);
        im=displayedContact("External friend");if(im==null||!Integer.valueOf(99).equals(((Hashtable)field(im.getClass(),"g",Hashtable.class).get(im)).get("2")))throw new AssertionError("IM setting operations changed offline contact identity");
        Files.writeString(out.resolve("phone-external-proof.txt"),"Phone: native t.d=+9607654321, t.e=null.\nExternal: native t.f[2]=friend@example.test, t.g[2]=99, t.e=null, dynamic local-directory icon2.\nBoth402 responses correlated to original405 builders and visible in original Friends Canvas.\nOriginal IM menu explicitly labels saved contacts (local);206 acknowledges only local directory availability and explicitly states no external sign-in;302/907 acknowledge while preserving local directory; no207providerconnection state received.\n");
    }
    static void receiver()throws Exception {
        verifyChat(1,"alice",direct,"private-recipient");checkPacket(1,direct,"alice",(short)1);mark("private-verified");
        waitGroup();mark("group-ready");verifyChat(2,group,grouped,"group-recipient");checkPacket(2,grouped,"alice",(short)1);mark("group-verified");
        peerMirror("origin","group-verified");contacts();mark("contacts-verified");peerMirror("origin","finished");
    }
    public static void main(String[] args){try{
        role=args[0];transport=args[1];user=role.equals("bob")?"bob":"alice";out=mirrorRoot.resolve(role);Connector.out=out;
        V4ContactIntegration.start();
        until("local directory startup acknowledged",()->Connector.count("TX",206)>=4&&received(1,transaction("TX",206)));
        if(Connector.count("RX",0)!=0||Connector.count("RX",207)!=0)throw new AssertionError("Startup produced errors or external provider connection state");
        if(role.equals("origin"))origin();else if(role.equals("mirror"))mirror();else receiver();logout();
        Files.writeString(out.resolve("result.txt"),"PASS original v4.62 "+role+" over "+transport+": own private/group mirrors, exact single local echo, preserved peer routing; Bob additionally verified phone/external contact models.\n");System.exit(0);
    }catch(Throwable error){error.printStackTrace();try{save("failure");}catch(Throwable ignored){}System.exit(1);}}
}
