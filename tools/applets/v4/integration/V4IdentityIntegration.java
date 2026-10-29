import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.*;
import org.microemu.MIDletBridge;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;

/** Original registration forms, activation and password-change browser pages. */
public class V4IdentityIntegration extends V4RoomDiscoveryIntegration {
    static final Path identityRoot = Paths.get("data/applets/integration/identity-out");
    static final String firstPassword = "Register &+=50% value";
    static final String changedPassword = "Changed &admin=true+50% value";
    static String identityPhase, mobile;

    static void boot() throws Exception {
        Files.createDirectories(out);
        System.setProperty("java.awt.headless", "true");
        System.setProperty("microedition.platform", "Loopback identity integration MicroEmulator");
        try { Connector.open("http://integration-proof.invalid"); throw new AssertionError("Remote connection allowed"); }
        catch (javax.microedition.io.ConnectionNotFoundException expected) { }
        Headless headless = new Headless(); Common emu = (Common)get(headless,"emulator");
        org.microemu.EmulatorContext ctx = (org.microemu.EmulatorContext)get(headless,"context");
        emu.initParams(new ArrayList<String>(Arrays.asList("--rms","memory","--appclassloader","system","main.Main")),
            new org.microemu.app.util.DeviceEntry("Default device",null,DeviceImpl.DEFAULT_LOCATION,true,false),J2SEDevice.class);
        J2SEDeviceDisplay dd = (J2SEDeviceDisplay)ctx.getDeviceDisplay();
        dd.setDisplayRectangle(new Rectangle(0,0,240,320)); dd.setDisplayPaintable(new Rectangle(0,0,240,320));
        emu.initMIDlet(true); Thread.sleep(1600);
        javax.microedition.midlet.MIDlet midlet = MIDletBridge.getCurrentMIDlet();
        canvas = Display.getDisplay(midlet).getCurrent(); cc=Class.forName("aj"); bc=Class.forName("bc");
        field(cc,"c",String.class).set(null,"4.62"); field(cc,"a",String.class).set(null,".000");
        field(cc,"b",int.class).setInt(null,"http".equals(transport)?1:2);
        field(bc,"e",String.class).set(null,"127.0.0.1"); field(bc,"f",String.class).set(null,"9119");
        field(bc,"d",String.class).set(null,"http://127.0.0.1:8080/");
        method(cc,"c",void.class).invoke(canvas); network=field(cc,"ab",bc).get(canvas);
        Display.getDisplay(midlet).setCurrent((Canvas)canvas);
        until("original signed-out Login Canvas",()->field(cc,"M",int.class).getInt(null)==15&&Display.getDisplay(midlet).getCurrent()==canvas);
        save("signed-out-login");
    }

    static Object nativeForm() throws Exception { return get(canvas,"aj"); }
    static int nativeRow(int labelID,int kind) throws Exception {
        Object form=nativeForm(); int count=(Integer)method(form.getClass(),"e",int.class).invoke(form);
        for(int i=0;i<count;i++) if ((Integer)method(form.getClass(),"b",int.class,int.class).invoke(form,i)==kind &&
            label(labelID).equals(method(form.getClass(),"b",String.class,int.class).invoke(form,i))) return i;
        throw new AssertionError("Missing native row label="+label(labelID)+" kind="+kind+" state="+field(cc,"M",int.class).getInt(null));
    }
    static void nativeValue(int labelID,int kind,String value) throws Exception {
        Object form=nativeForm(); method(form.getClass(),"a",void.class,int.class,String.class).invoke(form,nativeRow(labelID,kind),value);
    }
    static void nativeFocus(int labelID,int kind,String capture) throws Exception {
        Object form=nativeForm(); field(form.getClass(),"l",int.class).setInt(form,nativeRow(labelID,kind)); save(capture);
    }
    static void fireNative() throws Exception {
        method(cc,"keyPressed",void.class,int.class).invoke(canvas,((Canvas)canvas).getKeyCode(Canvas.FIRE));
    }
    static void nativeLogin(String password,String capture) throws Exception {
        clearDialogs(); method(cc,"c",void.class).invoke(canvas); save(capture+"-form");
        nativeValue(328,0,user);nativeValue(227,1,password);nativeFocus(183,5,capture+"-submit");
        int complete=Connector.count("RX",403); request(200,201,()->fireNative());
        until("original 202/203 login and roster",()->Connector.count("RX",403)>complete&&get(canvas,"aG")==null);
        save(capture+"-result");
    }
    static void openIdentityPage(String route,String title,String capture) throws Exception {
        clearDialogs();request(922,923,()->method(cc,"a",void.class,String.class,String.class,boolean.class,boolean.class,Class.forName("bl")).invoke(canvas,
            "http://127.0.0.1:8080/sites/index.php?"+route,null,true,true,null));
        waitPage(title);save(capture);
    }
    static void register() throws Exception {
        nativeFocus(247,10,"register-link");fireNative();
        until("original registration step1",()->field(cc,"M",int.class).getInt(null)==62);save("register-step1");
        nativeValue(328,0,user);nativeFocus(391,5,"username-availability-submit");
        int starts=Connector.count("RX",102);fireNative();
        until("native username HTTP check and101/102",()->Connector.count("RX",102)>starts&&field(cc,"M",int.class).getInt(null)==30);
        save("register-step2-captcha");
        nativeValue(227,1,firstPassword);nativeValue(199,9,mobile);nativeValue(268,16,"123456");
        nativeFocus(304,5,"register-step2-submit");request(103,1,()->fireNative());
        if(!user.equals(textField(packet("TX",103),1))||!firstPassword.equals(textField(packet("TX",103),2))||!"123456".equals(textField(packet("TX",103),5)))
            throw new AssertionError("Native registration fields changed");
        Thread.sleep(250);save("registration-success-dialog");clearDialogs();
        nativeLogin(firstPassword,"first-login");
        Byte activated=(Byte)method(packet("RX",203).getClass(),"b",Byte.class,short.class).invoke(packet("RX",203),(short)2);
        if(activated==null||activated!=0)throw new AssertionError("Native account activated before204");
        until("original activation step3",()->field(cc,"M",int.class).getInt(null)==41);save("activation-step3");
        nativeFocus(387,0,"activation-code-row");
        // Use the actual generic editor and commit path. Activate reads cd's
        // value even though the verification row is a separate form model.
        method(cc,"a",void.class,String.class,String.class,int.class,int.class,int.class).invoke(canvas,label(387),"",320,0,27);
        TextBox editor=(TextBox)field(cc,"cd",TextBox.class).get(canvas);Display display=(Display)field(cc,"ae",Display.class).get(canvas);
        until("original verification editor",()->display.getCurrent()==editor);editor.setString("123456");command("bz");
        until("original activation Canvas restored",()->display.getCurrent()==canvas);
        nativeFocus(412,5,"activate-submit");
        // setCurrent can precede the original painter's command refresh. FIRE
        // dispatches bD, so wait for the focused button and a populated Canvas.
        until("original activation button ready",()->{
            save("activate-submit");
            java.awt.image.BufferedImage rendered=javax.imageio.ImageIO.read(out.resolve("activate-submit.png").toFile());
            Set<Integer> colors=new HashSet<>();
            for(int y=24;y<110;y++)for(int x=4;x<232;x++)colors.add(rendered.getRGB(x,y));
            return display.getCurrent()==canvas && field(cc,"M",int.class).getInt(null)==41 &&
                field(cc,"bD",Command.class).get(canvas)==field(cc,"bz",Command.class).get(canvas) && colors.size()>=3;
        });
        request(204,1,()->fireNative());
        until("original activated flag set",()->field(cc,"i",boolean.class).getBoolean(null));save("activation-success");clearDialogs();
        V4ContactIntegration.friends("registered-empty-friends");
        openIdentityPage("c=settings&a=change_password&v=midlet","Change Password","change-password-current");
        setText("password",firstPassword);postForm("New Password","verify-current-password");
        setText("password",changedPassword);postForm("Password Changed","save-new-password");
        String body=textField(packet("TX",922),3);
        if(!body.endsWith("password="+changedPassword.replace(" ","%20")))throw new AssertionError("Original raw password tail changed: "+body);
        Files.writeString(out.resolve("native-password-contract.txt"),"Original serializer preserved ampersand, equals, plus and percent in the final password field. Spaces became %20.\n");
    }
    static void returning() throws Exception {
        nativeLogin(changedPassword,"returning-login");
        Byte activated=(Byte)method(packet("RX",203).getClass(),"b",Byte.class,short.class).invoke(packet("RX",203),(short)2);
        if(activated==null||activated!=1)throw new AssertionError("Activation lost between clients");
        if(!mobile.equals(textField(packet("RX",203),1)))throw new AssertionError("Mobile lost between clients");
        V4ContactIntegration.friends("returning-empty-friends");
        openIdentityPage("c=profile&a=home&v=midlet","User Profile","returning-profile");
        if(!responseText(923,transaction("TX",922),(short)2).contains("Username: "+user))throw new AssertionError("New profile missing");
    }
    public static void main(String[] args) {
        try {
            user=args[0];transport=args[1];identityPhase=System.getenv("IDENTITY_PHASE");
            mobile=Files.readString(identityRoot.resolve("mobile.txt")).trim();out=identityRoot.resolve(identityPhase);Connector.out=out;
            boot();if("register".equals(identityPhase))register();else returning();logout();
            Files.writeString(out.resolve("result.txt"),"PASS original v4.62 identity "+identityPhase+" / "+user+" over "+transport+". Native forms, serializers, activation and persistent password change.\n");
            System.out.println("PASS identity "+identityPhase);System.exit(0);
        }catch(Throwable error){error.printStackTrace();try{save("failure");}catch(Throwable ignored){}System.exit(1);}
    }
}
