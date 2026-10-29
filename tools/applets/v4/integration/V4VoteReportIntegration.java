import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.Canvas;

/** Native room menus, voting notices and Report Abuse browser forms. */
public class V4VoteReportIntegration extends V4BlockIntegration {
    static final Path voteRoot = Paths.get("data/applets/integration/vote-report-out");
    static final String reportDetails = "Repeated spam & flooding + 50% noise = test";
    static int detailsWidth;

    static void peer(String who, String marker) throws Exception {
        until(who + " reached " + marker, () -> Files.exists(voteRoot.resolve(who).resolve(marker)));
    }

    static void clearDialogs() throws Exception {
        while (!((Vector) get(canvas, "aI")).isEmpty()) method(cc, "j", void.class).invoke(canvas);
        method(cc, "k", void.class).invoke(canvas);
    }

    static int roomNoticeCount(String body) throws Exception {
        List<String> files = new ArrayList<>();
        synchronized (Connector.events) {
            for (String event : Connector.events) {
                String[] parts = event.split("\t");
                if (parts.length >= 5 && parts[0].equals("RX") && parts[1].equals("500")) files.add(parts[4]);
            }
        }
        int count = 0;
        Class<?> ay = Class.forName("ay");
        for (String file : files) {
            Object[] packets = (Object[]) method(ay, "a", java.lang.reflect.Array.newInstance(ay, 0).getClass(), byte[].class).invoke(null, Files.readAllBytes(out.resolve(file)));
            for (Object packet : packets) if (body.equals(textField(packet, 8))) count++;
        }
        return count;
    }

    static void orderedLogout() throws Exception {
        // Original bc.n makes the next received packet terminate the MIDlet
        // after300, even if it is another user's presence/leave notification.
        // Drain the preceding logout's offline event before sending the next.
        List<String> order = Arrays.asList("hailam", "bob", "alice", "carol", "dave");
        int position = order.indexOf(user);
        if (position > 0) {
            String previous = order.get(position - 1);
            peer(previous, "logged-out");
            Object contact = V4ContactIntegration.contact(previous);
            if (contact == null) throw new AssertionError("Logout fixture lacks contact " + previous);
            int id = V4ContactIntegration.id(contact, "a");
            until("preceding native offline event drained for " + previous, () -> {
                if (Connector.count("RX", 404) == 0) return false;
                Object p = packet("RX", 404);
                Integer who = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 1);
                Byte presence = (Byte) method(p.getClass(), "b", Byte.class, short.class).invoke(p, (short) 2);
                return who != null && who == id && presence != null && presence == 99;
            });
        }
        logout(); mark("logged-out");
    }

    static void nativeKick(String target, String capture) throws Exception {
        clearDialogs(); selectRoom("Lobby"); command("bw");
        selectMenu(label(171), capture + "-menu");
        int before = Connector.count("RX", 708);
        command("bD");
        until("original Kick User participant reply", () -> Connector.count("RX", 708) > before);
        until("original Kick User picker ready", () -> {
            Object picker = method(cc, "i", Class.forName("y")).invoke(canvas);
            return picker != null && field(picker.getClass(), "d", int.class).getInt(picker) == 45;
        });
        selectMenu(target, capture + "-picker"); command("bD");
        until("original kick confirmation", () -> !((Vector) get(canvas, "aI")).isEmpty());
        String prompt = dialogText();
        if (!prompt.contains("kick " + target)) throw new AssertionError("Unexpected kick prompt: " + prompt);
        save(capture + "-confirmation");
        Files.writeString(out.resolve(capture + "-confirmation.txt"), prompt + "\n");
        request(706, 1, () -> command("bD"));
        Object sent = packet("TX", 706);
        if (!"Lobby".equals(textField(sent, 1)) || !target.equals(textField(sent, 2)))
            throw new AssertionError("Native Kick User did not emit expected706 room/target");
        System.out.println("PASS original Kick User menu/selector/confirmation emitted706 as " + user);
    }

    static Object reportForm() throws Exception {
        Object page = page();
        if (page == null) return null;
        for (Object candidate : (Vector) field(page.getClass(), "y", Vector.class).get(page)) {
            if (candidate.getClass().getName().equals("s") && hidden(candidate).containsKey("csrf")) return candidate;
        }
        return null;
    }

    static void setSelect(String name, String value) throws Exception {
        Object page = page();
        for (Object control : (Vector) field(page.getClass(), "m", Vector.class).get(page)) {
            if (field(control.getClass(), "b", int.class).getInt(control) != 6) continue;
            Object select = field(control.getClass(), "k", Class.forName("bj")).get(control);
            if (!name.equals(field(select.getClass(), "b", String.class).get(select))) continue;
            Vector options = (Vector) field(select.getClass(), "f", Vector.class).get(select);
            for (int i = 0; i < options.size(); i++) {
                Object option = options.elementAt(i);
                if (value.equals(field(option.getClass(), "a", String.class).get(option))) {
                    field(select.getClass(), "f", int.class).setInt(select, i);
                    return;
                }
            }
            throw new AssertionError("Native select " + name + " lacks " + value);
        }
        throw new AssertionError("Native page lacks select " + name);
    }

    static void setDetails(String value) throws Exception {
        Object page = page();
        for (Object control : (Vector) field(page.getClass(), "m", Vector.class).get(page)) {
            if (field(control.getClass(), "b", int.class).getInt(control) != 8) continue;
            Object input = field(control.getClass(), "a", Class.forName("bj")).get(control);
            if ("details".equals(field(input.getClass(), "a", String.class).get(input))) {
                detailsWidth = field(input.getClass(), "b", int.class).getInt(input);
                int expectedWidth = field(Class.forName("bl"), "b", int.class).getInt(null) - 4;
                if (detailsWidth != expectedWidth || detailsWidth < 200)
                    throw new AssertionError("Native report textarea is not full width: " + detailsWidth + " expected " + expectedWidth);
                field(input.getClass(), "c", String.class).set(input, value);
                return;
            }
        }
        throw new AssertionError("Native page lacks report details textarea");
    }

    static void nativeReport() throws Exception {
        clearDialogs(); selectRoom("Lobby"); command("bw");
        selectMenu(label(259), "report-room-menu");
        request(922, 923, () -> command("bD"));
        until("native Report Abuse page parsed", () -> page() != null && "Report Abuse".equals(method(browser().getClass(), "a", String.class).invoke(browser())));
        until("native report form parsed", () -> reportForm() != null);
        save("report-form");
        Object form = reportForm();
        String action = (String) field(form.getClass(), "h", String.class).get(form);
        if (!action.startsWith("http://127.0.0.1:8080/sites/index.php?") || action.contains("&amp;") || !action.contains("c=report") || !action.contains("subject_id=Lobby"))
            throw new AssertionError("Invalid native report action: " + action);
        setSelect("category", "spam"); setSelect("target", "bob"); setDetails(reportDetails);
        Object page = page();
        int formIndex = ((Vector) field(page.getClass(), "y", Vector.class).get(page)).indexOf(form);
        Vector controls = (Vector) field(page.getClass(), "m", Vector.class).get(page);
        Vector focus = (Vector) field(page.getClass(), "n", Vector.class).get(page);
        int submit = -1;
        for (int i = 0; i < focus.size(); i++) {
            Object ref = focus.elementAt(i);
            Object control = controls.elementAt(field(ref.getClass(), "b", int.class).getInt(ref));
            if (field(control.getClass(), "b", int.class).getInt(control) == 2 && field(control.getClass(), "e", int.class).getInt(control) == formIndex) submit = i;
        }
        if (submit < 0) throw new AssertionError("Report form lacks native Submit control");
        field(page.getClass(), "r", int.class).setInt(page, submit);
        String[] serialized = (String[]) method(browser().getClass(), "f", String[].class).invoke(browser());
        if (serialized == null || !action.equals(serialized[1]) || !serialized[2].contains("target=bob") || !serialized[2].contains("category=spam") || !serialized[2].endsWith("details=" + reportDetails.replace(" ", "%20")))
            throw new AssertionError("Native report serialization differs from form contract");
        save("report-submit-focused");
        Files.writeString(out.resolve("report-form-model.txt"), "URL: " + action + "\nTarget: bob\nCategory: spam\nDetails: " + reportDetails + "\nNative textarea width: " + detailsWidth + " pixels (full page width minus 4).\nHidden csrf/nonce/binding preserved from original parser; values redacted here.\n");
        request(922, 923, () -> method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE)));
        Object sent = packet("TX", 922);
        if (!serialized[2].equals(textField(sent, 3))) throw new AssertionError("Original POST body was changed before transmission");
        Object response = packet("RX", 923);
        if (!Integer.valueOf(200).equals(method(response.getClass(), "d", Integer.class, short.class).invoke(response, (short) 1)))
            throw new AssertionError("Report POST returned failure: " + textField(response, 2));
        until("original report confirmation rendered", () -> page() != null && "Report Submitted".equals(method(browser().getClass(), "a", String.class).invoke(browser())));
        save("report-submitted");
        String html = textField(response, 2);
        if (!html.contains("saved for local review")) throw new AssertionError("Missing persisted-report confirmation: " + html);
        Files.writeString(out.resolve("report-confirmation.html"), html);
        System.out.println("PASS native Report Abuse menu, parsed form and FIRE POST with punctuation-heavy details");
        roomParticipants("Lobby", "report-does-not-kick", "alice", "bob", "carol", "dave", "hailam");
    }

    static void notification(int count, String expected, String capture) throws Exception {
        until("original kick notification " + count, () -> Connector.count("RX", 5) >= count && !((Vector) get(canvas, "aL")).isEmpty());
        selectRoom("Lobby"); save(capture);
        until("original kick popup visible", () -> !((Vector) get(canvas, "aI")).isEmpty());
        String body = dialogText();
        if (!body.contains(expected)) throw new AssertionError("Wrong kick notification: " + body);
        Files.writeString(out.resolve(capture + "-model.txt"), body + "\n");
        command("bS");
        until("kick popup dismissed", () -> ((Vector) get(canvas, "aL")).isEmpty() && ((Vector) get(canvas, "aI")).isEmpty());
    }

    static void hailam() throws Exception {
        for (String peer : new String[]{"alice", "bob", "carol", "dave"}) peer(peer, "joined");
        nativeReport();
        nativeKick("bob", "start-vote"); expectRoom("Lobby", "1/3", "vote-started");
        nativeKick("bob", "duplicate-vote");
        until("duplicate vote notice", () -> wireHasRoomText("Lobby", "already voted"));
        roomParticipants("Lobby", "duplicate-still-five-members", "alice", "bob", "carol", "dave", "hailam");
        mark("first-and-duplicate-votes");
        peer("dave", "third-vote");
        expectRoom("Lobby", "3/3", "vote-passed");
        roomParticipants("Lobby", "after-vote-kick", "alice", "carol", "dave", "hailam");
        peer("bob", "vote-send-denied");
        sendRoom("Lobby", "After vote kick must not reach Bob", 1); mark("post-vote-message");
        peer("bob", "moderator-send-denied"); peer("alice", "instant-kick");
        until("both Bob departure notices drained", () -> roomNoticeCount("bob left the room.") == 2);
        mark("finished");
    }

    static void carol() throws Exception {
        peer("hailam", "first-and-duplicate-votes"); nativeKick("bob", "second-vote");
        expectRoom("Lobby", "2/3", "second-vote-count");
        roomParticipants("Lobby", "two-votes-still-five-members", "alice", "bob", "carol", "dave", "hailam");
        mark("second-vote"); peer("hailam", "finished");
    }

    static void dave() throws Exception {
        peer("carol", "second-vote"); nativeKick("bob", "third-vote"); mark("third-vote");
        expectRoom("Lobby", "3/3", "third-vote-count"); peer("hailam", "finished");
    }

    static void bob() throws Exception {
        notification(1, "You were kicked from Lobby by a room vote.", "vote-kick-notification");
        sendRoom("Lobby", "DENIED after room vote", 0); clearDialogs(); mark("vote-send-denied");
        peer("hailam", "post-vote-message");
        if (wireHasRoomText("Lobby", "After vote kick must not reach Bob")) throw new AssertionError("Vote-kicked user received later room text");
        join("Lobby"); mark("rejoined");
        notification(2, "You were kicked from Lobby by alice.", "moderator-kick-notification");
        sendRoom("Lobby", "DENIED after moderator kick", 0); clearDialogs(); mark("moderator-send-denied");
        peer("hailam", "finished");
    }

    static void alice() throws Exception {
        peer("bob", "rejoined"); nativeKick("bob", "moderator-instant-kick");
        roomParticipants("Lobby", "after-moderator-kick", "alice", "carol", "dave", "hailam");
        mark("instant-kick"); peer("hailam", "finished");
    }

    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; out = voteRoot.resolve(user); Connector.out = out;
            V4ContactIntegration.start(); join("Lobby"); mark("joined");
            switch (user) {
                case "hailam": hailam(); break;
                case "bob": bob(); break;
                case "carol": carol(); break;
                case "dave": dave(); break;
                case "alice": alice(); break;
                default: throw new AssertionError("Unknown test user");
            }
            orderedLogout();
            Files.writeString(out.resolve("result.txt"), "PASS original v4.62 vote/report integration: " + user + " over " + transport + ".\n");
            System.exit(0);
        } catch (Throwable error) {
            error.printStackTrace();
            try { save("failure"); } catch (Throwable ignored) { }
            try { if (network != null && bc != null && field(bc, "c", boolean.class).getBoolean(null)) logout(); } catch (Throwable ignored) { }
            System.exit(1);
        }
    }
}
