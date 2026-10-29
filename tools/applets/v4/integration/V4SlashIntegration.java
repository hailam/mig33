import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.Command;
import javax.microedition.lcdui.Displayable;
import javax.microedition.lcdui.Display;
import javax.microedition.lcdui.TextBox;

/** Slash actions through the original editor, builder and incoming renderer. */
public class V4SlashIntegration extends V4RoomDiscoveryIntegration {
    static final Path slashRoot = Paths.get("data/applets/integration/slash-out");
    static final String[] commands = {"/me tests native action rendering", "/hug bob", "/slap bob", "/wave carol", "/smile", "/laugh", "/cry", "/brb", "/roll", "/fortune", "/8ball"};
    static final String privateBody = "alice checks private action routing";
    static final String offlineBody = "alice queues an offline action";

    static void peer(String who, String marker) throws Exception {
        until(who + " reached " + marker, () -> Files.exists(slashRoot.resolve(who).resolve(marker)));
    }

    static int privateIndex(String who) throws Exception {
        return (Integer) method(tabs().getClass(), "a", int.class, String.class).invoke(tabs(), who);
    }

    static Object conversation(int kind, String destination) throws Exception {
        if (kind == 3) return roomChat(destination);
        if (kind == 2) return groupChat();
        int tab = privateIndex(destination);
        return tab < 0 ? null : method(tabs().getClass(), "c", Class.forName("s"), int.class).invoke(tabs(), tab);
    }

    static void selectConversation(int kind, String destination) throws Exception {
        clearDialogs();
        if (kind == 3) selectRoom(destination);
        else if (kind == 2) selectGroup();
        else {
            int tab = privateIndex(destination);
            if (tab < 0) privateTab(destination);
            method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), privateIndex(destination));
        }
    }

    static String visibleRows(int kind, String destination) throws Exception {
        Object chat = conversation(kind, destination);
        if (chat == null) return "";
        StringBuilder text = new StringBuilder();
        Vector rows = (Vector) field(chat.getClass(), "y", Vector.class).get(chat);
        synchronized (rows) {
            for (Object row : rows) {
                String line = row.toString().replaceAll("µ[^µ]*µ", "").replaceAll("▲[^▲]*▲", "").replaceAll("♣[^♣]*♣", "");
                line = line.replaceAll("¤-?\\d+¤([^¤]*)¤", "$1");
                text.append(line).append(' ');
            }
        }
        return text.toString().replaceAll("\\s+", " ").trim();
    }

    static List<Object> incoming(int kind, String body) throws Exception {
        List<String> files = new ArrayList<>();
        synchronized (Connector.events) {
            for (String event : Connector.events) {
                String[] parts = event.split("\t");
                if (parts.length >= 5 && parts[0].equals("RX") && parts[1].equals("500")) files.add(parts[4]);
            }
        }
        List<Object> found = new ArrayList<>(); Class<?> ay = Class.forName("ay");
        for (String file : files) {
            Object[] packets = (Object[]) method(ay, "a", java.lang.reflect.Array.newInstance(ay, 0).getClass(), byte[].class).invoke(null, Files.readAllBytes(out.resolve(file)));
            for (Object packet : packets) {
                Byte conversation = (Byte) method(ay, "b", Byte.class, short.class).invoke(packet, (short) 3);
                if (conversation != null && conversation == kind && body.equals(textField(packet, 8))) found.add(packet);
            }
        }
        return found;
    }

    static void nativeSend(int kind, String destination, String text, int response, String capture) throws Exception {
        selectConversation(kind, destination);
        // A selected text color must not disable content-6 action rendering.
        field(conversation(kind, destination).getClass(), "e", int.class).setInt(conversation(kind, destination), 0x336699);
        command("bw"); selectMenu(label(342), capture + "-write-menu"); command("bD");
        if (field(cc, "aO", int.class).getInt(null) != 7) throw new AssertionError("Write did not open original chat editor");
        TextBox editor = (TextBox) field(cc, "ce", TextBox.class).get(canvas);
        Display display = (Display) field(cc, "ae", Display.class).get(canvas);
        // Display.setCurrent is asynchronous in MicroEmulator. The original
        // handler dispatches by getCurrent(), not its Displayable argument.
        until("original text editor became current", () -> display.getCurrent() == editor);
        editor.setString(text);
        int tx = request(500, response, () -> {
            Command send = (Command) field(cc, "bA", Command.class).get(canvas);
            method(cc, "commandAction", void.class, Command.class, Displayable.class).invoke(canvas, send, editor);
        });
        until("original Canvas restored after Send", () -> display.getCurrent() == canvas);
        if (!text.equals(textField(packet("TX", 500), 8))) throw new AssertionError("Native editor changed slash text");
        Integer color = (Integer) method(packet("TX", 500).getClass(), "d", Integer.class, short.class).invoke(packet("TX", 500), (short) 13);
        if (color == null || color != 0x336699) throw new AssertionError("Native selected color missing from command request");
        Files.writeString(out.resolve(capture + "-editor.txt"), text + "\ntransaction=" + tx + "\n");
        selectConversation(kind, destination);
    }

    static void assertAction(int kind, String destination, String body, String sender, String capture) throws Exception {
        until("original action packet and row " + body, () -> incoming(kind, body).size() > 0 && visibleRows(kind, destination).contains(body));
        List<Object> packets = incoming(kind, body);
        if (packets.size() != 1) throw new AssertionError("Action echo count " + packets.size() + " for " + body);
        Object packet = packets.get(0);
        Short content = (Short) method(packet.getClass(), "c", Short.class, short.class).invoke(packet, (short) 6);
        Integer color = (Integer) method(packet.getClass(), "d", Integer.class, short.class).invoke(packet, (short) 13);
        if (content == null || content != 6 || color != null || !sender.equals(textField(packet, 2)))
            throw new AssertionError("Wrong content6/default-color/sender contract for " + body);
        String rows = visibleRows(kind, destination);
        if (rows.indexOf(body) != rows.lastIndexOf(body) || rows.contains("alice: " + body) || rows.contains("Alice: " + body) || rows.contains("bob: " + body))
            throw new AssertionError("Duplicate action or sender prefix: " + rows);
        selectConversation(kind, destination); save(capture); Files.writeString(out.resolve(capture + "-model.txt"), rows + "\n");
    }

    static String latestActionBody() throws Exception {
        Object p = packet("RX", 500);
        Short type = (Short) method(p.getClass(), "c", Short.class, short.class).invoke(p, (short) 6);
        return type != null && type == 6 ? textField(p, 8) : null;
    }

    static void waitOffline(String who) throws Exception {
        peer(who, "logged-out"); Object row = V4ContactIntegration.contact(who);
        int id = V4ContactIntegration.id(row, "a");
        until(who + " offline presence drained", () -> {
            if (Connector.count("RX", 404) == 0) return false;
            Object p = packet("RX", 404);
            Integer contact = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 1);
            Byte presence = (Byte) method(p.getClass(), "b", Byte.class, short.class).invoke(p, (short) 2);
            return contact != null && contact == id && presence != null && presence == 99;
        });
    }

    static void alice() throws Exception {
        peer("bob", "joined"); peer("carol", "joined");
        for (int i = 0; i < commands.length; i++) {
            String capture = String.format("room-action-%02d", i);
            int before = Connector.count("RX", 500);
            nativeSend(3, "Lobby", commands[i], 1, capture);
            until("sender received action echo", () -> Connector.count("RX", 500) > before && latestActionBody() != null);
            String body = latestActionBody();
            if (!body.startsWith("alice ") || body.contains(commands[i])) throw new AssertionError("Server did not expand command: " + body);
            assertAction(3, "Lobby", body, "alice", capture);
            if (visibleRows(3, "Lobby").contains(commands[i])) throw new AssertionError("Original editor locally echoed slash command");
            Files.writeString(slashRoot.resolve(capture + ".txt"), body);
            peer("bob", capture); peer("carol", capture);
        }
        nativeSend(3, "Lobby", "/definitely_missing", 0, "unknown-command");
        until("original inline command error", () -> visibleRows(3, "Lobby").contains("Unsupported command")); save("unknown-command-error");
        nativeSend(3, "Lobby", "/hug missing", 0, "invalid-target");
        until("original inline target error", () -> visibleRows(3, "Lobby").contains("one username in this conversation")); save("invalid-target-error");
        nativeSend(3, "Lobby", "/kick bob", 0, "unavailable-kick");
        until("original unavailable kick error", () -> visibleRows(3, "Lobby").contains("Slash /kick is unavailable")); save("unavailable-kick-error");
        roomParticipants("Lobby", "members-unchanged-after-errors", "alice", "bob", "carol");
        nativeSend(1, "bob", "/me checks private action routing", 1, "private-action");
        assertAction(1, "bob", privateBody, "bob", "private-own-action-echo");
        if (privateIndex("alice") >= 0) throw new AssertionError("Own DM action echo opened self-chat"); mark("private-action");
        peer("bob", "private-action"); peer("carol", "private-isolated");
        selectConversation(1, "bob");
        int tx = request(751, 750, () -> method(bc, "a", void.class, String.class, String.class, String.class, byte.class).invoke(network, "bob", "carol", null, (byte) 1));
        group = responseText(750, tx, (short) 1); Files.writeString(slashRoot.resolve("group-id.txt"), group);
        until("original group tab", () -> groupTab() >= 0); peer("bob", "group-ready"); peer("carol", "group-ready");
        nativeSend(2, group, "/wave bob", 1, "group-action"); assertAction(2, group, "alice waves to bob.", "alice", "group-own-action-echo"); mark("group-action");
        peer("bob", "group-action"); peer("carol", "group-action"); mark("finished-live");
        waitOffline("bob"); waitOffline("carol");
        nativeSend(1, "bob", "/me queues an offline action", 1, "offline-action");
        assertAction(1, "bob", offlineBody, "bob", "offline-own-action-echo");
        if (privateIndex("alice") >= 0) throw new AssertionError("Offline own echo opened self-chat");
    }

    static void member() throws Exception {
        for (int i = 0; i < commands.length; i++) {
            String capture = String.format("room-action-%02d", i); Path result = slashRoot.resolve(capture + ".txt");
            until("room command result " + i, () -> Files.exists(result));
            assertAction(3, "Lobby", Files.readString(result), "alice", capture);
            if (!incoming(3, commands[i]).isEmpty() || visibleRows(3, "Lobby").contains(commands[i])) throw new AssertionError("Recipient received raw slash text");
            mark(capture);
        }
        peer("alice", "private-action");
        if (user.equals("bob")) { assertAction(1, "alice", privateBody, "alice", "private-action-from-alice"); mark("private-action"); }
        else { if (!incoming(1, privateBody).isEmpty()) throw new AssertionError("Private action leaked to Carol"); mark("private-isolated"); }
        until("group ID ready", () -> Files.exists(slashRoot.resolve("group-id.txt"))); group = Files.readString(slashRoot.resolve("group-id.txt"));
        until("original invited group tab", () -> groupTab() >= 0); mark("group-ready"); peer("alice", "group-action");
        assertAction(2, group, "alice waves to bob.", "alice", "group-action-from-alice"); mark("group-action"); peer("alice", "finished-live");
        if (user.equals("carol")) waitOffline("bob");
    }

    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; boolean returning = "returning".equals(System.getenv("SLASH_PHASE"));
            out = slashRoot.resolve(returning ? "returning-bob" : user); Connector.out = out; V4ContactIntegration.start();
            if (returning) {
                assertAction(1, "alice", offlineBody, "alice", "offline-action-delivered-after-login");
                if (!incoming(1, "/me queues an offline action").isEmpty()) throw new AssertionError("Offline delivery replayed raw slash");
            } else {
                join("Lobby"); mark("joined"); if (user.equals("alice")) alice(); else member();
            }
            logout(); mark("logged-out"); Files.writeString(out.resolve("result.txt"), "PASS original v4.62 slash actions: " + (returning ? "returning bob" : user) + " over " + transport + ".\n");
            System.out.println("PASS slash actions"); System.exit(0);
        } catch (Throwable error) {
            error.printStackTrace(); try { save("failure"); } catch (Throwable ignored) { }
            try { if (network != null && bc != null && field(bc, "c", boolean.class).getBoolean(null)) logout(); } catch (Throwable ignored) { }
            System.exit(1);
        }
    }
}
