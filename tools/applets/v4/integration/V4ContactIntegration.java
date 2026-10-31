import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.*;
import org.microemu.MIDletBridge;
import org.microemu.app.*;
import org.microemu.device.impl.*;
import org.microemu.device.j2se.*;

/** Original contact forms/builders/dispatcher, with two isolated client JVMs. */
public class V4ContactIntegration extends V4GroupIntegration {
    static final Path contactRoot = Paths.get("data/applets/integration/contact-out");
    static final String initialStatus = "Status from 402";
    static final String updatedStatus = "Status from 421";

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

    // Exercise the publisher's builders, dispatcher, renderer model and RMS
    // serializer; no generated contract or modern client interprets the fields.
    static void publishStatus(String value) throws Exception {
        request(601, 1, () -> method(bc, "c", void.class, String.class, String.class).invoke(network, value, ""));
    }
    static void publishAvatar(String key) throws Exception {
        request(602, 1, () -> method(bc, "d", void.class, String.class, String.class).invoke(network, "overwritten-tag-one", key));
    }
    static long newestGalleryPhoto() throws Exception {
        java.util.List<String> responses = new ArrayList<>();
        synchronized (Connector.events) {
            for (String event : Connector.events) {
                String[] parts = event.split("\t");
                if (parts.length >= 5 && parts[0].equals("RX") && parts[1].equals("923")) responses.add(parts[4]);
            }
        }
        Class<?> ay = Class.forName("ay");
        long newest = 0;
        for (String response : responses) {
            Object[] packets = (Object[]) method(ay, "a", java.lang.reflect.Array.newInstance(ay, 0).getClass(), byte[].class)
                .invoke(null, Files.readAllBytes(out.resolve(response)));
            for (Object packet : packets) {
                String html = (String) method(ay, "f", String.class, short.class).invoke(packet, (short) 2);
                if (html == null) continue;
                Matcher photos = Pattern.compile("/assets/photos/(\\d+)").matcher(html);
                while (photos.find()) newest = Math.max(newest, Long.parseLong(photos.group(1)));
            }
        }
        return newest;
    }
    static String uploadAvatar(int color) throws Exception {
        java.awt.image.BufferedImage image = new java.awt.image.BufferedImage(12, 12, java.awt.image.BufferedImage.TYPE_INT_RGB);
        for (int y = 0; y < 12; y++) for (int x = 0; x < 12; x++) image.setRGB(x, y, color);
        java.io.ByteArrayOutputStream bytes = new java.io.ByteArrayOutputStream();
        javax.imageio.ImageIO.write(image, "png", bytes);
        long previous = newestGalleryPhoto();
        request(504, 1, () -> method(bc, "a", void.class, short.class, byte[].class, short.class, byte.class)
            .invoke(network, (short) 1, bytes.toByteArray(), (short) -1, (byte) 0));
        // The native504 success dispatcher opens its gallery and also issues
        // image fetches. Inspect gallery HTML rather than the last922 request.
        until("original gallery returned uploaded avatar", () -> newestGalleryPhoto() > previous);
        return "photos/" + newestGalleryPhoto();
    }
    static String text(Object row, String name) throws Exception {
        return (String) field(row.getClass(), name, String.class).get(row);
    }
    static Object cachedBob() throws Exception {
        javax.microedition.rms.RecordStore store;
        try { store = javax.microedition.rms.RecordStore.openRecordStore("31", false); }
        catch (javax.microedition.rms.RecordStoreNotFoundException absent) { return null; }
        try {
            javax.microedition.rms.RecordEnumeration records = store.enumerateRecords(null, null, false);
            try {
                while (records.hasNextElement()) {
                    Object row = method(roster().getClass(), "a", Class.forName("t"), byte[].class).invoke(null, records.nextRecord());
                    if (row != null && "bob".equals(text(row, "e"))) return row;
                }
            } finally { records.destroy(); }
            return null;
        } finally { store.closeRecordStore(); }
    }
    static void expectFields(String status, String avatar, String capture) throws Exception {
        until("original roster " + capture, () -> contact("bob") != null && status.equals(text(contact("bob"), "h")) && avatar.equals(text(contact("bob"), "p")));
        until("original RMS cache " + capture, () -> {
            Object cached = cachedBob();
            return cached != null && status.equals(text(cached, "h")) && avatar.equals(text(cached, "p"));
        });
        Object row = contact("bob");
        byte[] encoded = (byte[]) method(row.getClass(), "a", byte[].class, row.getClass()).invoke(null, row);
        Object restored = method(roster().getClass(), "a", row.getClass(), byte[].class).invoke(null, encoded);
        if (!status.equals(text(restored, "h")) || !avatar.equals(text(restored, "p")))
            throw new AssertionError("Original contact RMS round trip swapped status/avatar");
        Files.write(out.resolve(capture + "-rms.bin"), encoded);
        friends(capture);
        Files.writeString(out.resolve(capture + "-fields.tsv"), "source\tstatus_t.h\tavatar_t.p\nroster\t" + text(row, "h") + "\t" + text(row, "p")
            + "\nrms-roundtrip\t" + text(restored, "h") + "\t" + text(restored, "p")
            + "\nrms-recordstore-31\t" + text(cachedBob(), "h") + "\t" + text(cachedBob(), "p") + "\n");
    }
    static void expectFieldsPacket(int opcode, String status, String avatar) throws Exception {
        Object packet = V4BlockIntegration.packet("RX", opcode);
        int contactID = (Integer) method(packet.getClass(), "d", Integer.class, short.class).invoke(packet, (short) 1);
        if (contactID != id(contact("bob"), "a")) throw new AssertionError("Wrong contact ID in " + opcode);
        String first = (String) method(packet.getClass(), "f", String.class, short.class).invoke(packet, (short) (opcode == 402 ? 27 : 2));
        String expected = opcode == 423 ? avatar : status;
        if (!expected.equals(first)) throw new AssertionError("Wrong status/avatar wire slot in " + opcode + ": " + first);
        if (opcode == 402 && !avatar.equals(method(packet.getClass(), "f", String.class, short.class).invoke(packet, (short) 28)))
            throw new AssertionError("402 tag28 must be the avatar key");
    }
    static void contactFieldUpdates() throws Exception {
        String firstAvatar = Files.readString(contactRoot.resolve("bob/initial-avatar.txt")).trim();
        expectFields(initialStatus, firstAvatar, "contact-fields-402");
        expectFieldsPacket(402, initialStatus, firstAvatar);
        mark("fields-402-checked");
        peer("bob", "status-updated");
        until("original421 status push", () -> Connector.count("RX", 421) > 0);
        expectFields(updatedStatus, firstAvatar, "contact-fields-421");
        expectFieldsPacket(421, updatedStatus, firstAvatar);
        mark("fields-421-checked");
        peer("bob", "avatar-updated");
        String nextAvatar = Files.readString(contactRoot.resolve("bob/updated-avatar.txt")).trim();
        if (firstAvatar.equals(nextAvatar)) throw new AssertionError("Avatar fixtures must be distinct");
        until("original423 avatar push", () -> Connector.count("RX", 423) > 0);
        expectFields(updatedStatus, nextAvatar, "contact-fields-423");
        expectFieldsPacket(423, updatedStatus, nextAvatar);
        mark("fields-423-checked");
    }
    static void hailam() throws Exception {
        peer("bob", "fields-ready");
        if (contact("bob") != null) throw new AssertionError("Use a fresh backend database: Hailam should initially have an empty address book");
        nativeAddBob("added-bob"); contactFieldUpdates(); removeBob(); nativeAddBob("readded-bob");
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
        String firstAvatar = uploadAvatar(0xc06040);
        publishStatus(initialStatus); publishAvatar(firstAvatar);
        Files.writeString(out.resolve("initial-avatar.txt"), firstAvatar + "\n"); mark("fields-ready");
        peer("hailam", "fields-402-checked");
        publishStatus(updatedStatus); mark("status-updated");
        peer("hailam", "fields-421-checked");
        String nextAvatar = uploadAvatar(0x40a060);
        publishAvatar(nextAvatar);
        Files.writeString(out.resolve("updated-avatar.txt"), nextAvatar + "\n"); mark("avatar-updated");
        peer("hailam", "fields-423-checked");
        friends("after-profile-updates");
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
            Files.writeString(out.resolve("result.txt"), "PASS original v4.62 contact integration: " + user + " over " + transport + "; distinct402 status/tag27 and avatar/tag28, independent421 status and423 avatar pushes, original roster/RMS/Canvas.\n");
            System.exit(0);
        } catch (Throwable error) { error.printStackTrace(); try { save("failure"); } catch (Throwable ignored) {} System.exit(1); }
    }
}
