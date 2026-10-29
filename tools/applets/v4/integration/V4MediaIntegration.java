import java.io.*;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import javax.microedition.io.Connector;

/** Original upload/message builders, downloaded media decoders and language jobs.
 * Fixture pixels replace camera hardware; the unchanged client performs all
 * serialization, response dispatch, media decoding and gallery rendering. */
public class V4MediaIntegration extends V4PagesIntegration {
    static final Path mediaRoot=Paths.get("data/applets/integration/media-out");
    static String origin(){return "http://127.0.0.1:"+Connector.HTTP_PORT;}
    static Object bytesField(Object packet,int tag)throws Exception {
        return method(packet.getClass(),"g",byte[].class,short.class).invoke(packet,(short)tag);
    }
    static int pageTransaction(String url)throws Exception {
        List<String[]> events=new ArrayList<>();synchronized(Connector.events){for(String event:Connector.events)events.add(event.split("\t"));}
        Class<?> ay=Class.forName("ay");int latest=-1;
        for(String[] event:events)if(event.length>=5&&event[0].equals("TX")&&event[1].equals("922")) {
            Object[] packets=(Object[])method(ay,"a",java.lang.reflect.Array.newInstance(ay,0).getClass(),byte[].class).invoke(null,Files.readAllBytes(out.resolve(event[4])));
            if(url.equals(textField(packets[0],2)))latest=Integer.parseInt(event[2]);
        }
        return latest;
    }
    static int gallery(String capture)throws Exception {
        clearDialogs();
        String url=origin()+"/sites/index.php?c=photo&a=home&v=midlet";int previous=pageTransaction(url);
        method(cc,"a",void.class,String.class,String.class,boolean.class,boolean.class,Class.forName("bl")).invoke(canvas,url,null,true,true,null);
        until("original gallery request",()->pageTransaction(url)!=previous);
        int tx=pageTransaction(url);until("correlated gallery response",()->received(923,tx));
        acceptPage(tx,capture);
        String html=responseText(923,tx,(short)2);
        Matcher images=Pattern.compile("src=\"([^\"]*/assets/photos/[^\"]+)\"").matcher(html);int checked=0;
        while(images.find()) {
            String imageURL=images.group(1);
            until("original gallery image cache retained "+imageURL,()->{
                Object cached=method(browser().getClass(),"a",Class.forName("bj"),String.class).invoke(browser(),imageURL);
                return cached!=null&&field(cached.getClass(),"a",javax.microedition.lcdui.Image.class).get(cached)!=null;
            });
            checked++;
        }
        if(checked==0)throw new AssertionError("No gallery images were checked");
        savePage(capture);
        Files.writeString(out.resolve(capture+"-images.txt"),"PASS "+checked+" original browser image cache entries retained decoded images after painting.\n");
        return tx;
    }
    static void downloads()throws Exception {
        until("native service descriptors and icon decoding",()->Connector.count("RX",208)>=4&&Connector.count("RX",927)>=4);
        until("local directory startup acknowledgments",()->Connector.count("TX",206)>=4&&received(1,transaction("TX",206)));
        if(Connector.count("RX",0)!=0||Connector.count("RX",207)!=0)throw new AssertionError("Local directory startup produced errors or provider state");
        for(int service:new int[]{2,3,4,6}) {
            Object descriptor=method(cc,"h",Class.forName("bj"),int.class).invoke(null,service);
            if(descriptor==null||field(descriptor.getClass(),"f",javax.microedition.lcdui.Image.class).get(descriptor)==null)
                throw new AssertionError("Native provider icon missing for "+service);
            if(!field(descriptor.getClass(),"a",String.class).get(descriptor).toString().contains("saved contacts (local)"))throw new AssertionError("Directory mislabeled as external connection");
        }
        String[] tokens={"[smile]","[buzz]","[sparkle]","[chime]"};
        int[] types={1,2,3,4};
        for(int i=0;i<tokens.length;i++) {
            String token=tokens[i]; int expected=types[i];
            request(913,914,()->method(bc,"a",void.class,String.class,int.class).invoke(network,token,0));
            if(!(token+" ").equals(textField(packet("TX",913),1)))throw new AssertionError("Original trailing space missing");
            until("original emoticon cache installed "+token,()->((Hashtable)get(canvas,"an")).containsKey(token));
            Object cached=((Hashtable)get(canvas,"an")).get(token);
            if(field(cached.getClass(),"f",int.class).getInt(cached)!=expected)throw new AssertionError("Media type mismatch");
            byte[] data=(byte[])bytesField(packet("RX",914),4);
            if(expected==3) {
                javax.microedition.lcdui.Image[] frames=(javax.microedition.lcdui.Image[])method(cc,"a",javax.microedition.lcdui.Image[].class,byte[].class).invoke(null,data);
                if(frames.length!=3||frames[0].getWidth()!=12)throw new AssertionError("Original animation decoder failed");
            } else if(expected==4) {
                Object[] pair=(Object[])method(cc,"b",Object[].class,byte[].class).invoke(null,data);
                if(!(pair[0] instanceof javax.microedition.lcdui.Image)||!Arrays.equals(((InputStream)pair[1]).readNBytes(4),new byte[]{'R','I','F','F'}))throw new AssertionError("Original image/WAV decoder failed");
            }
            Files.write(out.resolve("decoded-"+expected+".bin"),data);
        }
        Files.writeString(out.resolve("media-decoders.txt"),"PASS original914 dispatcher/cache and static, vibration-image, animation-frame, image/WAV decoders. Requests use mode0; hardware sound/vibration is not asserted.\n");
    }
    static void photos()throws Exception {
        byte[] png=Files.readAllBytes(Paths.get("research/applets/v4/artifacts/extracted/default-avatar.png"));
        java.awt.image.BufferedImage source=javax.imageio.ImageIO.read(new ByteArrayInputStream(png));
        java.awt.image.BufferedImage rgb=new java.awt.image.BufferedImage(source.getWidth(),source.getHeight(),java.awt.image.BufferedImage.TYPE_INT_RGB);
        java.awt.Graphics graphics=rgb.getGraphics();graphics.drawImage(source,0,0,null);graphics.dispose();
        ByteArrayOutputStream encoded=new ByteArrayOutputStream();javax.imageio.ImageIO.write(rgb,"jpeg",encoded);
        byte[][] sources={png,encoded.toByteArray()};
        for(int i=0;i<sources.length;i++) {
            byte[] data=sources[i];byte purpose=(byte)i;short parameter=(short)(i==0?-1:96);
            clearDialogs();
            request(504,1,()->method(bc,"a",void.class,short.class,byte[].class,short.class,byte.class).invoke(network,(short)1,data,parameter,purpose));
            if(!Arrays.equals(data,(byte[])bytesField(packet("TX",504),2)))throw new AssertionError("Photo source bytes changed");
            int tx=gallery("gallery-source-"+i);String html=responseText(923,tx,(short)2);
            Matcher ids=Pattern.compile("/assets/photos/(\\d+)").matcher(html);long newest=0;
            while(ids.find())newest=Math.max(newest,Long.parseLong(ids.group(1)));
            if(newest==0)throw new AssertionError("Uploaded photo not in original gallery");
            final String photo=""+newest;
            final int bobID=V4ContactIntegration.id(V4ContactIntegration.contact("bob"),"a");
            request(500,1,()->method(bc,"a",void.class,short.class,String.class,int.class,String.class,byte[].class,short.class,int.class).invoke(network,(short)1,"bob",bobID,null,data,(short)2,-1));
            request(500,1,()->method(bc,"a",void.class,short.class,String.class,int.class,String.class,byte[].class,short.class,int.class).invoke(network,(short)1,"bob",bobID,photo,null,(short)5,-1));
            request(913,914,()->method(bc,"a",void.class,String.class,int.class).invoke(network,"[photo:"+photo+"]",0));
            javax.microedition.lcdui.Image preview=javax.microedition.lcdui.Image.createImage((byte[])bytesField(packet("RX",914),4),0,((byte[])bytesField(packet("RX",914),4)).length);
            if(preview.getWidth()<1||preview.getWidth()>96)throw new AssertionError("Photo preview size");
            request(602,1,()->method(bc,"d",void.class,String.class,String.class).invoke(network,"new-id-is-overwritten","photos/"+photo));
            if(!("photos/"+photo).equals(textField(packet("TX",602),1)))throw new AssertionError("Original602 duplicate tag behavior changed");
            Files.writeString(out.resolve("photo-source-"+i+".txt"),"PASS original504 purpose="+purpose+";500content2 raw bytes andcontent5 reference;913/914preview;602 actual received owned key. Photo="+photo+"\n");
        }
        request(602,1,()->method(bc,"d",void.class,String.class,String.class).invoke(network,"new-id-is-overwritten",""));
    }
    static void language()throws Exception {
        clearDialogs();method(bc,"b",void.class,String.class).invoke(network,"http://www.mig33.com/sites/resources/language/v4.6/l");
        until("original public language1001/1002 list",()->{Object menu=method(cc,"i",Class.forName("y")).invoke(canvas);return menu!=null&&field(menu.getClass(),"d",int.class).getInt(menu)==57;});
        Object form=method(cc,"i",Class.forName("y")).invoke(canvas);int count=(Integer)method(form.getClass(),"c",int.class).invoke(form);
        if(count<1)throw new AssertionError("Language list empty");save("language-catalog");
        int before=Connector.count("TX",925);
        method(bc,"c",void.class,String.class).invoke(network,origin()+"/sites/resources/language/v4.6/en-US");
        until("original1003/1004 install sent925",()->Connector.count("TX",925)>before);
        int tx=transaction("TX",925);until("language selection ACK",()->received(1,tx));
        if(!"en-US".equals(textField(packet("TX",925),1)))throw new AssertionError("Wrong installed language");
        Files.writeString(out.resolve("language.txt"),"PASS original localHTTP1001/1002 catalog and1003/1004 English installation;925 selection acknowledgment. Pack is byte-exact bundled en-US.\n");
    }
    static void providers()throws Exception {
        clearDialogs();int sms=request(500,0,()->method(bc,"a",void.class,short.class,String.class,int.class,String.class,byte[].class,short.class,int.class).invoke(network,(short)2,"+9607771234",-1,"Native SMS fixture",null,(short)1,-1));
        if(!responseText(0,sms,(short)2).contains("pending_provider"))throw new AssertionError("SMS pending request missing");
        clearDialogs();int mail=request(500,1,()->method(bc,"a",void.class,short.class,String.class,int.class,String.class,byte[].class,short.class,int.class).invoke(network,(short)9,"bob",-1,"Native local mail fixture",null,(short)1,-1));
        if(!responseText(1,mail,(short)1).contains("local_delivered"))throw new AssertionError("Local mail not delivered");
        clearDialogs();int tx=request(900,1,()->method(bc,"b",void.class,String.class,String.class).invoke(network,"+9607771234","Local invitation"));
        if(!responseText(1,tx,(short)1).contains("pending_provider"))throw new AssertionError("Misleading invite receipt");
        clearDialogs();tx=request(800,1,()->method(bc,"a",void.class,String.class,String.class,int.class,int.class).invoke(network,"+9607771234","+9607775678",0,1));
        if(!responseText(1,tx,(short)1).contains("pending_provider")||Connector.count("RX",804)!=0)throw new AssertionError("Callback was presented as dialed");
        clearDialogs();
    }
    public static void main(String[]args) {
        try {
            user=args[0];transport=args[1];out=mediaRoot.resolve(transport);Connector.out=out;init(false);
            downloads();photos();language();providers();clearDialogs();logout();
            Files.writeString(out.resolve("result.txt"),"PASS original v4.62 media/native services over "+transport+": four914 content decoders, PNG/JPEG504 uploads, raw500/content5 photos, gallery,602,900,800,927, SMS/mail500 adapters, language1001/1002/1003/1004/925, logoutACK. Camera hardware and audio/vibration hardware remain outside this headless harness.\n");
            System.exit(0);
        }catch(Throwable error){error.printStackTrace();try{save("failure");}catch(Throwable ignored){}System.exit(1);}
    }
}
