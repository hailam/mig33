import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.*;
import org.microemu.MIDletBridge;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;

/** Original contact forms/builders/dispatcher, with two isolated client JVMs. */
public class V4ContactIntegration extends V4GroupIntegration {
    static final Path contactRoot = Paths.get("data/applets/integration/contact-out");

    static void peer(String who, String marker) throws Exception {
        until(who + " reached " + marker, () -> Files.exists(contactRoot.resolve(who).resolve(marker)));
    }

    static void start() throws Exception {
        Files.createDirectories(out);
        System.setProperty("java.awt.headless", "true");
        System.setProperty("microedition.platform", "Loopback contacts integration MicroEmulator");
        try { Connector.open("http://integration-proof.invalid"); throw new AssertionError("Remote connection allowed"); }
        catch (javax.microedition.io.ConnectionNotFoundException expected) { }
        Headless headless = new Headless();
        Common emu = (Common) get(headless, "emulator");
        org.microemu.EmulatorContext ctx = (org.microemu.EmulatorContext) get(headless, "context");
        emu.initParams(new ArrayList<String>(Arrays.asList("--rms", "memory", "--appclassloader", "system", "main.Main")),
            new org.microemu.app.util.DeviceEntry("Default device", null, DeviceImpl.DEFAULT_LOCATION, true, false), J2SEDevice.class);
        J2SEDeviceDisplay dd = (J2SEDeviceDisplay) ctx.getDeviceDisplay();
        dd.setDisplayRectangle(new Rectangle(0, 0, 240, 320));
        dd.setDisplayPaintable(new Rectangle(0, 0, 240, 320));
        emu.initMIDlet(true);
        Thread.sleep(1600);
        javax.microedition.midlet.MIDlet midlet = MIDletBridge.getCurrentMIDlet();
        canvas = Display.getDisplay(midlet).getCurrent();
        cc = Class.forName("aj"); bc = Class.forName("bc");
        Display.getDisplay(midlet).setCurrent(new Canvas() { public void paint(Graphics g) {} });
        field(cc, "c", String.class).set(null, "4.62");
        field(cc, "a", String.class).set(null, ".000");
        field(cc, "b", int.class).setInt(null, transport.equals("http") ? 1 : 2);
        field(bc, "e", String.class).set(null, "127.0.0.1");
        field(bc, "f", String.class).set(null, "9119");
        field(bc, "d", String.class).set(null, "http://127.0.0.1:8080/");
        method(cc, "c", void.class).invoke(canvas);
        network = field(cc, "ab", bc).get(canvas);
        method(cc, "f", void.class, String.class, String.class).invoke(canvas, user, password());
        until("original login and roster", () -> Connector.count("RX", 203) > 0 && Connector.count("RX", 403) > 0 && Connector.count("RX", 715) > 0);
        until("original login progress cleared", () -> get(canvas, "aG") == null);
        friends("initial-roster"); mark("ready");
    }

    static String password() { return user.equals("hailam") ? "hailam" : user + "-demo"; }
    static Object roster() throws Exception { return get(canvas, "Z"); }
    static Object contact(String name) throws Exception {
        Object roster = roster();
        int count = (Integer) method(roster.getClass(), "c", int.class).invoke(roster);
        for (int i = 0; i < count; i++) {
            Object row = method(roster.getClass(), "e", Class.forName("t"), int.class).invoke(roster, i);
            if (row != null && name.equals(field(row.getClass(), "e", String.class).get(row))) return row;
        }
        return null;
    }
    static Object contactGroup(String name) throws Exception {
        Vector groups = (Vector) method(roster().getClass(), "o", Vector.class).invoke(roster());
        for (Object row : groups) if (name.equals(field(row.getClass(), "m", String.class).get(row))) return row;
        return null;
    }
    static int id(Object row, String fieldName) throws Exception { return field(row.getClass(), fieldName, int.class).getInt(row); }
    static void selectContact(String name) throws Exception {
        Object row = contact(name);
        if (row == null) throw new AssertionError("Missing contact " + name);
        field(cc, "aq", Class.forName("t")).set(canvas, row);
    }
    static void command(String name) throws Exception {
        Command cmd = (Command) field(cc, name, Command.class).get(canvas);
        if (cmd == null) throw new AssertionError("Missing original command " + name);
        method(cc, "commandAction", void.class, Command.class, Displayable.class).invoke(canvas, cmd, canvas);
    }
    static void friends(String capture) throws Exception {
        method(cc, "k", void.class).invoke(canvas);
        int tab = (Integer) method(tabs().getClass(), "b", int.class, int.class).invoke(tabs(), 1);
        method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), tab);
        method(roster().getClass(), "f", void.class).invoke(roster()); // Original scroll-to-top action.
        save(capture);
        StringBuilder rows = new StringBuilder("kind\tcontact_id\tgroup_id\tusername\tdisplay\tgroup_name\n");
        int count = (Integer) method(roster().getClass(), "c", int.class).invoke(roster());
        for (int i = 0; i < count; i++) {
            Object row = method(roster().getClass(), "e", Class.forName("t"), int.class).invoke(roster(), i);
            rows.append(id(row, "l")).append('\t').append(id(row, "a")).append('\t').append(id(row, "b"));
            for (String name : new String[]{"e", "c", "m"}) rows.append('\t').append(field(row.getClass(), name, String.class).get(row));
            rows.append('\n');
        }
        Files.writeString(out.resolve(capture + "-model.tsv"), rows);
    }
    static void nativeAddBob(String capture) throws Exception {
        friends(capture + "-before");
        method(cc, "m", void.class, String.class).invoke(canvas, "bob");
        Object form = get(canvas, "aj");
        int rows = (Integer) method(form.getClass(), "e", int.class).invoke(form);
        // Focus the original Add control; all validation and serialization remain native.
        field(form.getClass(), "l", int.class).setInt(form, rows - 1);
        save(capture + "-form");
        request(405, 402, () -> command("bz"));
        until("Bob added by original form and dispatcher", () -> contact("bob") != null);
        friends(capture);
    }
    static void removeBob() throws Exception {
        selectContact("bob"); int contactID = id(contact("bob"), "a");
        request(406, 1, () -> method(bc, "a", void.class, short.class, int.class).invoke(network, (short) 406, contactID));
        until("original delete ACK removes Bob", () -> contact("bob") == null);
        friends("after-delete-bob");
    }
    static void reauthenticate() throws Exception {
        int before = Connector.count("TX", 419), complete = Connector.count("RX", 403);
        method(cc, "a", void.class, String.class).invoke(canvas, "Connecting...");
        method(bc, "a", void.class, String.class, String.class).invoke(network, user, password());
        until("original reauthentication chooses419", () -> Connector.count("TX", 419) > before);
        until("incremental sync completed", () -> Connector.count("RX", 403) > complete && get(canvas, "aG") == null);
        if (contact("bob") == null || !"Robert Local".equals(field(contact("bob").getClass(), "c", String.class).get(contact("bob"))))
            throw new AssertionError("Reauthentication lost edited Bob contact");
        friends("after-incremental-relogin");
    }
    static void hailam() throws Exception {
        peer("bob", "ready");
        if (contact("bob") != null) throw new AssertionError("Use a fresh backend database: Hailam should initially have an empty address book");
        nativeAddBob("added-bob"); removeBob(); nativeAddBob("readded-bob");
        request(408, 401, () -> method(bc, "j", void.class, String.class).invoke(network, "Contact Test"));
        until("new original contact group", () -> contactGroup("Contact Test") != null);
        int groupID = id(contactGroup("Contact Test"), "b");
        field(cc, "ap", Class.forName("t")).set(canvas, contactGroup("Contact Test"));
        request(410, 1, () -> method(bc, "b", void.class, String.class, int.class).invoke(network, "Local Friends", groupID));
        until("original renamed group", () -> contactGroup("Local Friends") != null && contactGroup("Contact Test") == null);
        selectContact("bob"); int bobID = id(contact("bob"), "a");
        request(411, 1, () -> method(bc, "b", void.class, int.class, int.class).invoke(network, bobID, groupID));
        until("original Bob moved into custom group", () -> id(contact("bob"), "b") == groupID);
        selectContact("bob");
        request(407, 402, () -> method(bc, "a", void.class, String.class, String.class, String.class, Vector.class, Vector.class, int.class, int.class)
            .invoke(network, "Robert Local", null, "bob", null, null, bobID, groupID));
        until("original edit response renamed Bob", () -> "Robert Local".equals(field(contact("bob").getClass(), "c", String.class).get(contact("bob"))));
        friends("edited-contact-custom-group");
        int friendsID = id(contactGroup("Friends"), "b");
        selectContact("bob");
        request(411, 1, () -> method(bc, "b", void.class, int.class, int.class).invoke(network, bobID, friendsID));
        until("Bob moved back to Friends", () -> id(contact("bob"), "b") == friendsID);
        field(cc, "ap", Class.forName("t")).set(canvas, contactGroup("Local Friends"));
        request(409, 1, () -> method(bc, "a", void.class, short.class, int.class).invoke(network, (short) 409, groupID));
        until("original ACK deleted empty custom group", () -> contactGroup("Local Friends") == null);
        friends("after-group-cleanup");
        reauthenticate();
        request(500, 1, () -> send("bob", "Contact test from Hailam"));
        peer("bob", "message-received");
        until("reply reached Hailam original chat", () -> privateTabFor("Contact test reply from Bob") >= 0);
        method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), privateTabFor("Contact test reply from Bob"));
        save("chat-after-add-contact"); mark("finished");
    }
    static void bob() throws Exception {
        until("Hailam message reached Bob original chat", () -> privateTabFor("Contact test from Hailam") >= 0);
        if (contact("hailam") == null) throw new AssertionError("Hailam's edits changed Bob's independent address book");
        friends("independent-roster");
        method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), privateTabFor("Contact test from Hailam"));
        save("message-from-new-contact");
        request(500, 1, () -> send("hailam", "Contact test reply from Bob"));
        mark("message-received"); peer("hailam", "finished");
    }
    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; out = contactRoot.resolve(user); Connector.out = out;
            start(); if (user.equals("hailam")) hailam(); else bob(); logout();
            Files.writeString(out.resolve("result.txt"), "PASS original v4.62 contact integration: " + user + " over " + transport + ".\n");
            System.exit(0);
        } catch (Throwable error) { error.printStackTrace(); try { save("failure"); } catch (Throwable ignored) {} System.exit(1); }
    }
}
