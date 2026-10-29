import java.io.*;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.io.Connector;

/** Unchanged4.62 serializers, page parser and Canvas against reconstructed formatting services. */
public class V4FormattingIntegration extends V4PagesIntegration {
    static final Path formattingRoot = Paths.get("data/applets/integration/formatting-out");
    static final String origin = "http://127.0.0.1:" + Connector.HTTP_PORT;
    static final String profileRoute = "/sites/index.php?c=profile&a=home&v=midlet&username=alice";
    static final String editRoute = "/sites/index.php?c=profile&a=edit&v=midlet&field=customization";
    static final String red = "WAP red text";
    static final String mixed = "Mixed red blue";
    static final String green = "WAP green group";
    static final String blue = "WAP blue room";
    static final String literal = "[color=red]Literal[/color]";

    static void peerFormatting(String who, String marker) throws Exception {
        until(who + " reached " + marker, () -> {
            if (Files.exists(formattingRoot.resolve(who).resolve("failed")))
                throw new AssertionError(who + " failed; inspect its runtime.log");
            return Files.exists(formattingRoot.resolve(who).resolve(marker));
        });
    }

    static String open(String url, String capture) throws Exception {
        clearDialogs();
        String absolute = url.startsWith("http") ? url : origin + url;
        int tx = pageRequest(absolute, null, () -> method(cc, "a", void.class, String.class, String.class,
            boolean.class, boolean.class, Class.forName("bl")).invoke(canvas, absolute, null, true, true, null));
        String html = acceptPage(tx, capture); opened++;
        Files.writeString(out.resolve("pages.tsv"), capture + "\t" + absolute + "\t" + tx + "\t200\n",
            StandardOpenOption.CREATE, StandardOpenOption.APPEND);
        return html;
    }

    static String post(String button, String capture) throws Exception {
        until("native submit " + button, () -> submitFocus(button) >= 0);
        field(page().getClass(), "r", int.class).setInt(page(), submitFocus(button));
        String[] form = (String[]) method(browser().getClass(), "f", String[].class).invoke(browser());
        if (form == null || form[2] == null || !form[1].startsWith(origin + "/") || form[1].contains("&amp;"))
            throw new AssertionError("Wrong native form serialization: " + Arrays.toString(form));
        savePage(capture + "-before");
        int tx = pageRequest(form[1], form[2], () -> firePage());
        Files.writeString(out.resolve(capture + "-submitted.txt"), form[1] + "\n" + form[2] + "\n");
        posted++;
        return acceptPage(tx, capture);
    }

    // A page can immediately enqueue image922 requests. Correlate to the page
    // URL/body, never to the last generic922 transaction observed by a poll.
    static int pageRequest(String url, String body, Action action) throws Exception {
        final int start;
        synchronized(Connector.events){start=Connector.events.size();}
        action.run();
        final int[] selected={-1};
        until("native page request " + url,()->{
            List<String> events;
            synchronized(Connector.events){events=new ArrayList<>(Connector.events.subList(start,Connector.events.size()));}
            for(String event:events){
                String[] parts=event.split("\t");
                if(parts.length<5||!parts[0].equals("TX")||!parts[1].equals("922"))continue;
                Class<?> ay=Class.forName("ay");
                Object[] packets=(Object[])method(ay,"a",java.lang.reflect.Array.newInstance(ay,0).getClass(),byte[].class)
                    .invoke(null,Files.readAllBytes(out.resolve(parts[4])));
                if(url.equals(textField(packets[0],2)) && Objects.equals(body,textField(packets[0],3)))
                    selected[0]=Integer.parseInt(parts[2]);
            }
            return selected[0]>=0;
        });
        until("correlated page923",()->received(923,selected[0]));
        return selected[0];
    }

    static int pixels(String capture, int colour) throws Exception {
        BufferedImage image = ImageIO.read(out.resolve(capture + ".png").toFile());
        int count = 0;
        for (int y = 24; y < 306; y++) for (int x = 4; x < 232; x++)
            if ((image.getRGB(x,y) & 0xffffff) == colour) count++;
        return count;
    }

    static void verifyPixels(String capture, int colour, int minimum) throws Exception {
        int count = pixels(capture, colour);
        if (count < minimum) throw new AssertionError("Canvas " + capture + " only has " + count +
            " pixels of #" + Integer.toHexString(colour) + "; expected at least " + minimum);
        Files.writeString(out.resolve("colour-pixels.tsv"), capture + "\t" + Integer.toHexString(colour) +
            "\t" + count + "\n", StandardOpenOption.CREATE, StandardOpenOption.APPEND);
    }

    // Emulator fonts are antialiased: foreground ink is blended with white,
    // so only a few glyph pixels equal the exact protocol RGB value.
    static void verifyTextColour(String capture,int colour) throws Exception {
        BufferedImage image=ImageIO.read(out.resolve(capture+".png").toFile());
        int dominant=colour==0xff0000?0:colour==0x008000?1:2, count=0;
        for(int y=24;y<306;y++)for(int x=25;x<232;x++){
            int p=image.getRGB(x,y);int[] channels={(p>>16)&255,(p>>8)&255,p&255};
            if(channels[dominant]-channels[(dominant+1)%3]>40 && channels[dominant]-channels[(dominant+2)%3]>40)count++;
        }
        if(count<15)throw new AssertionError("Canvas "+capture+" lacks colour ink: "+count);
        Files.writeString(out.resolve("colour-ink.tsv"),capture+"\t"+Integer.toHexString(colour)+"\t"+count+"\n",
            StandardOpenOption.CREATE,StandardOpenOption.APPEND);
    }

    static void saveChat(String capture) throws Exception {
        long deadline = System.currentTimeMillis() + 15000, previous = 0;
        boolean readyBefore = false;
        while (System.currentTimeMillis() < deadline) {
            save(capture);
            BufferedImage image = ImageIO.read(out.resolve(capture + ".png").toFile());
            Set<Integer> colors = new HashSet<>(); long hash = 1125899906842597L;
            for (int y = 24; y < 306; y++) for (int x = 4; x < 232; x++) {
                int rgb = image.getRGB(x,y) & 0xffffff; colors.add(rgb); hash = 31 * hash + rgb;
            }
            boolean ready = colors.size() >= 3 && !field(tabs().getClass(), "c", boolean.class).getBoolean(tabs());
            if (ready && readyBefore && previous == hash) return;
            previous = hash; readyBefore = ready; Thread.sleep(80);
        }
        throw new AssertionError("Unstable chat Canvas " + capture);
    }

    static void expectFormatted(int kind, String target, String body, Integer rgb, String capture) throws Exception {
        until("formatted packet " + body, () -> incoming(kind, body).size() > 0 && conversation(kind,target) != null);
        List<Object> packets = incoming(kind, body);
        if (packets.size() != 1) throw new AssertionError("Expected one packet for " + body + ": " + packets.size());
        Object p = packets.get(0);
        Integer actual = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p,(short)13);
        if (!Objects.equals(actual,rgb) || !"alice".equals(textField(p,2)) || !body.equals(textField(p,8)))
            throw new AssertionError("Wrong trusted native fallback: " + textField(p,2) + "/" + textField(p,8) + "/" + actual);
        String rows = visibleRows(kind,target);
        if (rows.contains("<p") || rows.contains("<span") || rows.contains("<b>"))
            throw new AssertionError("Raw markup leaked into native chat: " + rows);
        selectConversation(kind,target); saveChat(capture);
        if (rgb != null) verifyTextColour(capture,rgb);
        Files.writeString(out.resolve(capture + "-model.txt"),rows + "\n");
    }

    static long uploadBackground() throws Exception {
        BufferedImage source = new BufferedImage(24,24,BufferedImage.TYPE_INT_RGB);
        for (int y=0;y<24;y++) for (int x=0;x<24;x++) source.setRGB(x,y, ((x/6+y/6)%2==0) ? 0xbbedff : 0x98c8e8);
        ByteArrayOutputStream bytes = new ByteArrayOutputStream(); ImageIO.write(source,"png",bytes);
        byte[] data = bytes.toByteArray(); Files.write(out.resolve("background-fixture.png"),data);
        request(504,1,()->method(bc,"a",void.class,short.class,byte[].class,short.class,byte.class)
            .invoke(network,(short)1,data,(short)-1,(byte)0));
        String html = open("/sites/index.php?c=photo&a=home&v=midlet","owned-background-photo");
        Matcher ids = Pattern.compile("/assets/photos/(\\d+)").matcher(html); long newest=0;
        while(ids.find()) newest=Math.max(newest,Long.parseLong(ids.group(1)));
        if(newest==0)throw new AssertionError("Native504 photo absent from gallery");
        return newest;
    }

    static String wap(String kind, String target) {
        return "/member2/t.php?kind=" + kind + "&target=" + target;
    }

    static String nativeTextValue(String name) throws Exception {
        for(Object control:(Vector)field(page().getClass(),"m",Vector.class).get(page())){
            int kind=field(control.getClass(),"b",int.class).getInt(control);
            if(kind!=4&&kind!=8)continue;
            Object value=field(control.getClass(),kind==4?"i":"a",Class.forName("bj")).get(control);
            if(name.equals(field(value.getClass(),kind==4?"c":"a",String.class).get(value)))
                return (String)field(value.getClass(),kind==4?"b":"c",String.class).get(value);
        }
        throw new AssertionError("Missing native field "+name);
    }

    static void sendWAP(String kind, String target, String html, String capture) throws Exception {
        open(wap(kind,target),capture + "-composer");
        setSelect("format","html");
        setFormText("Send","body",html);
        String result = post("Send",capture + "-sent");
        if (result.contains("could not") || result.contains("not allowed")) throw new AssertionError("WAP send failed: " + result);
    }

    static void aliceFormatting() throws Exception {
        peerFormatting("bob","ready");
        long photo = uploadBackground();
        String source = "<body bgcolor=\"#fff0cc\"><p style=\"color:#ff0000\"><b>Historical red profile &amp; welcome + 50% = yes</b></p>" +
            "<p><i>Italic</i> <u>underline</u></p><p><a href=\"mig33:privateChat(carol)\">Chat with Carol</a></p></body>";
        open(editRoute,"profile-customization-editor"); setText("value",source);
        String saved=post("Save","profile-customization-saved");
        checkContains(saved,"&lt;body","Escaped customization source");
        String replacement=nativeTextValue("value");
        if(replacement!=null&&!replacement.trim().isEmpty())throw new AssertionError("Replacement editor must start blank: "+replacement);
        String profile = open(profileRoute,"own-colour-profile");
        checkContains(profile,"Historical red profile","Customized profile");
        checkContains(profile,"&amp; welcome + 50% = yes","Source punctuation survived original serializer and safe rendering");
        verifyPixels("own-colour-profile",0xff0000,20); verifyPixels("own-colour-profile",0xfff0cc,1000);
        mark("profile-saved"); peerFormatting("bob","profile-viewed");

        source = "<body background=\"photo:" + photo + "\"><p><b>Photo background profile</b></p>" +
            "<img src=\"photo:" + photo + "\" width=\"32\" height=\"32\"></body>";
        open(editRoute,"profile-photo-editor"); setText("value",source); post("Save","profile-photo-saved");
        mark("photo-saved"); peerFormatting("bob","photo-viewed");

        sendWAP("private","bob","<p style=\"color:#ff0000\"><b>" + red + "</b></p>","wap-private-red");
        mark("private-red-sent"); peerFormatting("bob","private-red-received");
        sendWAP("private","bob","<p><span style=\"color:#ff0000\">Mixed red</span> <span style=\"color:#0000ff\">blue</span></p>","wap-private-mixed");
        mark("mixed-sent"); peerFormatting("bob","mixed-received");

        selectConversation(1,"bob");
        int tx=request(751,750,()->method(bc,"a",void.class,String.class,String.class,String.class,byte.class)
            .invoke(network,"bob","carol",null,(byte)1));
        group=responseText(750,tx,(short)1); Files.writeString(formattingRoot.resolve("group-id.txt"),group);
        mark("group-created"); peerFormatting("bob","group-ready");
        sendWAP("group",group,"<p style=\"color:#008000\"><u>" + green + "</u></p>","wap-group-green");
        mark("group-sent"); peerFormatting("bob","group-received");

        join("Lobby"); mark("room-joined"); peerFormatting("bob","room-joined");
        sendWAP("room","Lobby","<p style=\"color:#0000ff\"><i>" + blue + "</i></p>","wap-room-blue");
        mark("room-sent"); peerFormatting("bob","room-received");

        nativeSend(1,"bob",literal,1,"native-literal-send");
        mark("literal-sent"); peerFormatting("bob","literal-received");
        open(editRoute,"profile-reset-editor"); post("Reset","profile-reset-result");
        mark("profile-reset"); peerFormatting("bob","reset-viewed");
        mark("finished"); logout(); mark("logged-out");
    }

    static void bobFormatting() throws Exception {
        peerFormatting("alice","profile-saved");
        String html = open(profileRoute,"other-colour-profile");
        checkContains(html,"mig33:privateChat(carol)","Original native profile action");
        verifyPixels("other-colour-profile",0xff0000,20); verifyPixels("other-colour-profile",0xfff0cc,1000);
        until("custom native Chat link available",()->linkFocus("mig33:privateChat(carol)")>=0);
        field(page().getClass(),"r",int.class).setInt(page(),linkFocus("mig33:privateChat(carol)"));
        firePage();until("custom profile link opened Carol private tab",()->privateIndex("carol")>=0);
        if((Integer)method(tabs().getClass(),"i",int.class,int.class).invoke(tabs(),privateIndex("carol"))!=3)
            throw new AssertionError("Custom link did not open native private chat");
        save("profile-native-chat-link");
        mark("profile-viewed");
        peerFormatting("alice","photo-saved");
        String photoHTML=open(profileRoute,"other-photo-profile");
        Matcher photoURLs=Pattern.compile("(?:src|background)=\"([^\"]+/assets/photos/[^\"]+)\"").matcher(photoHTML);
        StringBuilder photoEvidence=new StringBuilder();
        while(photoURLs.find()){
            String url=photoURLs.group(1);
            if(url.endsWith("?w=32"))until("native inline photo decoded",()->{
                Object entry=method(browser().getClass(),"a",Class.forName("bj"),String.class).invoke(browser(),url);
                return entry!=null&&field(entry.getClass(),"a",javax.microedition.lcdui.Image.class).get(entry)!=null;
            });
            Object cached=method(browser().getClass(),"a",Class.forName("bj"),String.class).invoke(browser(),url);
            photoEvidence.append(url).append(" cache=").append(cached).append(" image=")
                .append(cached==null?null:field(cached.getClass(),"a",javax.microedition.lcdui.Image.class).get(cached)).append('\n');
        }
        Files.writeString(out.resolve("profile-images.txt"),photoEvidence.toString());
        savePage("other-photo-profile");
        verifyPixels("other-photo-profile",0xbbedff,200);
        Files.writeString(out.resolve("native-photo-observation.txt"),
            "Original Canvas fixture-colour pixels: "+pixels("other-photo-profile",0xbbedff)+
            ". PASS inline photo rendered. Original body background syntax is parsed but not fetched or painted; no background-image claim.\n");
        mark("photo-viewed");

        peerFormatting("alice","private-red-sent");
        expectFormatted(1,"alice",red,0xff0000,"native-private-red"); mark("private-red-received");
        peerFormatting("alice","mixed-sent");
        expectFormatted(1,"alice",mixed,null,"native-private-mixed-fallback");
        html = open(wap("private","alice"),"wap-private-rich-history");
        checkContains(html,"#ff0000","Rich red history"); checkContains(html,"#0000ff","Rich blue history");
        checkContains(html,"Mixed red","Rich mixed message"); mark("mixed-received");

        peerFormatting("alice","group-created"); group=Files.readString(formattingRoot.resolve("group-id.txt"));
        until("original group tab restored",()->groupTab()>=0); mark("group-ready");
        peerFormatting("alice","group-sent");
        expectFormatted(2,group,green,0x008000,"native-group-green"); mark("group-received");
        peerFormatting("alice","room-joined"); join("Lobby"); mark("room-joined");
        peerFormatting("alice","room-sent");
        expectFormatted(3,"Lobby",blue,0x0000ff,"native-room-blue"); mark("room-received");

        peerFormatting("alice","literal-sent");
        until("ordinary native BBCode remains literal",()->incoming(1,literal).size()==1);
        Object literalPacket=incoming(1,literal).get(0);
        if(!literal.equals(textField(literalPacket,8)))throw new AssertionError("Native text changed");
        selectConversation(1,"alice");saveChat("native-bbcode-literal");mark("literal-received");
        peerFormatting("alice","profile-reset"); html=open(profileRoute,"profile-after-reset");
        if(html.contains("Photo background profile")||html.contains("Historical red profile")||html.contains("background=\""))
            throw new AssertionError("Profile customization survived reset");
        mark("reset-viewed"); peerFormatting("alice","finished"); logout(); mark("logged-out");
    }

    public static void main(String[] args) {
        try {
            user=args[0];transport=args[1];out=formattingRoot.resolve(user);Connector.out=out;
            V4ContactIntegration.start();
            if(user.equals("alice"))aliceFormatting();else bobFormatting();
            Files.writeString(out.resolve("result.txt"),"PASS original v4.62 historical-formatting compatibility: "+user+"/"+transport+
                "; native profile forms, shared profile colours/inline photo, WAP rich messages and original private/group/room RGB fallback; pages="+opened+"; posts="+posted+".\n");
            System.exit(0);
        } catch(Throwable error) {
            error.printStackTrace();try{mark("failed");save("failure");}catch(Throwable ignored){}
            try{if(network!=null&&bc!=null&&field(bc,"c",boolean.class).getBoolean(null))logout();}catch(Throwable ignored){}
            System.exit(1);
        }
    }
}
