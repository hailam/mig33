import java.io.*;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import javax.microedition.io.Connector;

/** Catalog renditions exercised through unchanged4.62 forms/media/Canvas. */
public class V4CatalogIntegration extends V4FormattingIntegration {
    static final Path catalogRoot = Paths.get("data/applets/integration/catalog-out");
    static final String placeholder = "[pack:lovely-pack]";
    static final String giftToken = "[gift:big-hug]";
    static final String giftBody = "alice sent bob a Big Hug " + giftToken;
    static final String giftNote = "Native gift note & plus + equals =";
    static final String commandGiftBody = "alice sent bob a Kiss [gift:kiss] Message: " + giftNote;

    static void peerCatalog(String who, String marker) throws Exception {
        long deadline = System.currentTimeMillis() + 180000;
        while(System.currentTimeMillis() < deadline) {
            if (Files.exists(catalogRoot.resolve(who).resolve("failed")))
                throw new AssertionError(who + " failed; inspect runtime.log");
            if(Files.exists(catalogRoot.resolve(who).resolve(marker))) return;
            Thread.sleep(100);
        }
        throw new AssertionError("Timed out waiting for " + who + ": " + marker);
    }

    static String linkByLabel(String html, String label) {
        Matcher links = Pattern.compile("<a href=\"([^\"]+)\">([^<]+)</a>").matcher(html);
        while(links.find()) if(links.group(2).equals(label)) return links.group(1);
        throw new AssertionError("Missing link " + label);
    }

    static String follow(String html, String label, String capture) throws Exception {
        String href = linkByLabel(html, label);
        until("native link " + label, () -> linkFocus(href) >= 0);
        field(page().getClass(), "r", int.class).setInt(page(), linkFocus(href));
        int tx = pageRequest(href, null, () -> firePage());
        opened++;
        return acceptPage(tx, capture);
    }

    static void download(String token, int expected, String capture) throws Exception {
        clearDialogs();
        int tx = request(913, 914, () -> method(bc,"a",void.class,String.class,int.class).invoke(network, token, 0));
        until("original media cache " + token, () -> ((Hashtable)get(canvas,"an")).containsKey(token));
        Object cached = ((Hashtable)get(canvas,"an")).get(token);
        if(field(cached.getClass(),"f",int.class).getInt(cached) != expected)
            throw new AssertionError("Wrong native media type for " + token);
        Object p = packet("RX",914);
        byte[] data = (byte[])method(p.getClass(),"g",byte[].class,short.class).invoke(p,(short)4);
        if(expected == 3) {
            javax.microedition.lcdui.Image[] frames = (javax.microedition.lcdui.Image[])method(cc,"a",javax.microedition.lcdui.Image[].class,byte[].class).invoke(null,data);
            if(frames.length != 3 || frames[0].getWidth()!=12) throw new AssertionError("Original gift decoder failed");
        } else {
            javax.microedition.lcdui.Image img = javax.microedition.lcdui.Image.createImage(data,0,data.length);
            if(img.getWidth()!=12 || img.getHeight()!=12) throw new AssertionError("Wrong placeholder image dimensions");
        }
        Files.write(out.resolve(capture+".bin"),data);
        Files.writeString(out.resolve(capture+".txt"),"PASS original913/914 request/cache/decoder: "+token+" type="+expected+" transaction="+tx+". Recovered atlas placeholder, not original catalog download.\n");
    }

    static String allMessageBodies() throws Exception {
        StringBuilder text = new StringBuilder();
        List<String> paths = new ArrayList<>();
        synchronized(Connector.events) {
            for(String event:Connector.events) {
                String[] e=event.split("\t");
                if(e.length>=5&&e[0].equals("RX")&&e[1].equals("500")) paths.add(e[4]);
            }
        }
        Class<?> ay=Class.forName("ay");
        for(String path:paths) {
            Object[] packets=(Object[])method(ay,"a",java.lang.reflect.Array.newInstance(ay,0).getClass(),byte[].class).invoke(null,Files.readAllBytes(out.resolve(path)));
            for(Object packet:packets) text.append(textField(packet,8)).append('\n');
        }
        return text.toString();
    }

    static void aliceCatalog() throws Exception {
        peerCatalog("bob","ready");
        String html=open(url("store","home"),"store-first-page");
        checkContains(html,"Products 1–8 of 78","Bounded store catalog");
        html=follow(html,"Next products","store-second-page");
        checkContains(html,"Products 9–16 of 78","Next catalog page");
        html=follow(html,"Previous products","store-back-first-page");
        checkContains(html,"Products 1–8 of 78","Previous catalog page");
        html=open(url("account","recharge_credit"),"recharge-form");
        html=post("Add 10.00 test credits","recharge-complete");
        checkContains(html,"Added 10.00","Mock credit recharge");

        html=open(url("store","home")+"&item=emotes-lovely-pack","lovely-pack-detail");
        checkContains(html,"one local placeholder emoticon","Pack boundary visible");
        checkContains(html,"No, go back","Purchase confirmation cancel link");
        int oldCatalog=Connector.count("RX",916);
        html=post("Buy Lovely Pack","lovely-pack-purchased");
        checkContains(html,"Lovely Pack delivered to alice","Pack receipt");
        until("new picker entitlement",()->Connector.count("RX",916)>oldCatalog&&textField(packet("RX",916),1).contains(placeholder));
        Files.writeString(out.resolve("picker-entitlement.txt"),textField(packet("RX",916),1)+"\n");
        download(placeholder,1,"lovely-placeholder-decoded");

        html=open(url("store","home")+"&item=big-hug&username=bob","big-hug-detail");
        checkContains(html,"Historical gift name with a local placeholder animation","Gift boundary visible");
        checkContains(html,"Recipient: bob","Gift recipient");
        html=post("Buy Big Hug","big-hug-purchased");
        checkContains(html,"Big Hug delivered to bob","Gift receipt");
        mark("gift-sent");peerCatalog("bob","gift-received");

        nativeSend(1,"bob","/gift bob Kiss -m " + giftNote,1,"gift-send-command");
        until("native command purchase receipt and separator",()->allMessageBodies().contains("Gift sent: Kiss delivered to bob. Receipt #"));
        nativeSend(1,"bob","/gift rate Kiss 4",1,"gift-rate-command");
        until("native saved gift rating and separator",()->allMessageBodies().contains("Gift rating: Kiss: Your rating: 4/5. Local average: 4.0/5"));
        nativeSend(1,"bob","/gift display Kiss",1,"rated-gift-display-command");
        until("rated gift display",()->allMessageBodies().contains("Gift: Kiss"));
        selectConversation(1,"bob");saveChat("rated-gift-display");
        mark("command-gift-sent");peerCatalog("bob","command-gift-received");

        nativeSend(1,"bob",placeholder,1,"placeholder-message");
        mark("placeholder-sent");peerCatalog("bob","placeholder-received");

        html=open(url("bot","bot_list"),"games-catalog");
        checkContains(html,"Historical games","Historical game catalog");
        html=follow(html,"Moonbase (reference)","moonbase-reference");
        checkContains(html,"external game is unavailable here","Reference unavailable explanation");
        if(submitFocus("Play Moonbase")>=0) throw new AssertionError("Reference game has fake play control");

        html=open(url("chatroom","create"),"create-room-limit");
        checkContains(html,"15","Recovered new-room name limit");
        setText("name","Catalog Room 15");post("Continue","create-room-15-name");
        setText("description","Original native catalog acceptance");
        html=post("Create Room","create-room-15-complete");
        checkContains(html,"Catalog Room 15","Native15-character room creation");
        open(url("group","list_my_groups"),"bot-group-form");setText("name","Catalog Crew");
        post("Create Public Group","bot-group-name");setText("description","Local catalog bot lifecycle");
        post("Create Group","bot-group-created");
        open(url("group","add_to_group")+"&roomName=Catalog%20Room%2015","bot-room-link-form");
        html=post("Catalog Crew","bot-room-linked");checkContains(html,"Room links updated","Group-room link");
        join("Catalog Room 15");
        nativeSend(3,"Catalog Room 15","/bot dice",1,"bot-start-command");
        until("native bot start notice",()->allMessageBodies().contains("alice started the local dice bot"));
        nativeSend(3,"Catalog Room 15","/bot stop",1,"bot-stop-command");
        until("native two-word bot stop notice",()->allMessageBodies().contains("alice stopped the room bot."));
        selectConversation(3,"Catalog Room 15");saveChat("bot-stopped-result");
        join("Lobby");mark("joined-lobby");peerCatalog("bob","joined-lobby");
        nativeSend(3,"Lobby","/games",1,"games-command");
        until("private native game catalog",()->allMessageBodies().contains("Playable local replacements:"));
        selectConversation(3,"Lobby");saveChat("games-command-result");
        Files.writeString(out.resolve("games-command-model.txt"),visibleRows(3,"Lobby")+"\n");
        mark("games-listed");peerCatalog("bob","games-not-broadcast");
        nativeSend(3,"Lobby","/gift search hug",1,"gift-search-command");
        until("native gift search",()->allMessageBodies().contains("Gift search: hug"));
        selectConversation(3,"Lobby");saveChat("gift-search-result");
        nativeSend(3,"Lobby","/gift display Big Hug",1,"gift-display-command");
        until("native multiword gift details",()->allMessageBodies().contains("Gift: Big Hug"));
        selectConversation(3,"Lobby");saveChat("gift-display-result");
        mark("gifts-listed");peerCatalog("bob","gift-discovery-checked");
        mark("finished");peerCatalog("bob","logged-out");logout();mark("logged-out");
    }

    static void bobCatalog() throws Exception {
        peerCatalog("alice","gift-sent");
        until("gift receipt message",()->incoming(1,giftBody).size()==1&&conversation(1,"alice")!=null);
        Object receipt=incoming(1,giftBody).get(0);
        if(!"alice".equals(textField(receipt,2)))throw new AssertionError("Gift sender lost");
        download(giftToken,3,"big-hug-animation-decoded");
        selectConversation(1,"alice");saveChat("gift-receipt-chat");
        String html=open(url("store","home")+"&ty=1","received-gift-inventory");
        checkContains(html,"Big Hug from alice","Saved recipient inventory");
        mark("gift-received");
        peerCatalog("alice","command-gift-sent");
        until("single native command gift receipt",()->incoming(1,commandGiftBody).size()==1);
        download("[gift:kiss]",3,"kiss-animation-decoded");
        selectConversation(1,"alice");saveChat("command-gift-note-chat");
        Files.writeString(out.resolve("command-gift-note.txt"),textField(incoming(1,commandGiftBody).get(0),8)+"\n");
        html=open(url("store","home")+"&ty=1","received-gift-note-inventory");
        checkContains(html,"Native gift note &amp; plus + equals =","Safely rendered saved gift note");
        mark("command-gift-received");
        peerCatalog("alice","placeholder-sent");
        until("paid placeholder message",()->incoming(1,placeholder).size()==1);
        download(placeholder,1,"received-placeholder-decoded");
        selectConversation(1,"alice");saveChat("received-placeholder-chat");mark("placeholder-received");
        peerCatalog("alice","joined-lobby");join("Lobby");mark("joined-lobby");
        peerCatalog("alice","games-listed");
        Thread.sleep(500);
        if(allMessageBodies().contains("Playable local replacements:"))throw new AssertionError("Private /games response leaked to Bob");
        Files.writeString(out.resolve("games-requester-only.txt"),"PASS Alice's /games discovery was not delivered to Bob in the same room.\n");
        mark("games-not-broadcast");peerCatalog("alice","gifts-listed");
        if(allMessageBodies().contains("Gift search: hug")||allMessageBodies().contains("Gift: Big Hug"))throw new AssertionError("Private gift discovery leaked to Bob");
        nativeSend(1,"alice","/gift gifts",1,"received-gifts-command");
        until("native received-gifts discovery",()->allMessageBodies().contains("Your received gifts (newest first):") && allMessageBodies().contains("Kiss from alice") && allMessageBodies().contains("Big Hug from alice"));
        selectConversation(1,"alice");saveChat("received-gifts-result");
        Files.writeString(out.resolve("gift-discovery.txt"),"PASS search/detail remained requester-only, and Bob's /gift gifts listed his stored Big Hug receipt.\n");
        if(incoming(1,giftBody).size()!=1 || incoming(1,commandGiftBody).size()!=1)
            throw new AssertionError("Gift receipt was delivered more than once during the run");
        mark("gift-discovery-checked");peerCatalog("alice","finished");logout();mark("logged-out");
    }

    public static void main(String[]args) {
        try {
            user=args[0];transport=args[1];out=catalogRoot.resolve(user);Connector.out=out;
            V4ContactIntegration.start();
            if(user.equals("alice"))aliceCatalog();else bobCatalog();
            Files.writeString(out.resolve("result.txt"),"PASS original4.62 catalog compatibility "+user+"/"+transport+"; store pagination/confirmation, mock recharge, gift receipt, paid placeholder entitlement/download, reference games and requester-only /games and /gift discovery; pages="+opened+"; posts="+posted+".\n");
            System.exit(0);
        }catch(Throwable error){
            error.printStackTrace();try{mark("failed");save("failure");}catch(Throwable ignored){}
            try{if(network!=null&&bc!=null&&field(bc,"c",boolean.class).getBoolean(null))logout();}catch(Throwable ignored){}
            System.exit(1);
        }
    }
}
