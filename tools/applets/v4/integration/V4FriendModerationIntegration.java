import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.Canvas;

/** Original friend search/profile commands and browser room-moderation forms. */
public class V4FriendModerationIntegration extends V4RoomDiscoveryIntegration {
    static final Path friendRoot = Paths.get("data/applets/integration/friend-moderation-out");
    static final String queuedMessage = "Offline profile chat from Hailam";

    static void peer(String who, String marker) throws Exception {
        until(who + " reached " + marker, () -> Files.exists(friendRoot.resolve(who).resolve(marker)));
    }

    static void fire() throws Exception {
        method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE));
    }

    static void focusLink(String href, String capture) throws Exception {
        until("original browser link " + href, () -> linkFocus(href) >= 0);
        field(page().getClass(), "r", int.class).setInt(page(), linkFocus(href)); save(capture);
    }

    static String profileLink(String name) throws Exception {
        if (page() == null) return null;
        for (Object ref : (Vector) field(page().getClass(), "n", Vector.class).get(page())) {
            String href = (String) field(ref.getClass(), "c", String.class).get(ref);
            if (href != null && href.contains("c=profile") && href.contains("a=home") && href.endsWith("username=" + name)) return href;
        }
        return null;
    }

    static void selectBrowser(String title) throws Exception {
        Vector entries = (Vector) field(tabs().getClass(), "a", Vector.class).get(tabs());
        for (int i = 0; i < entries.size(); i++) {
            if ((Integer) method(tabs().getClass(), "i", int.class, int.class).invoke(tabs(), i) != 7) continue;
            Object candidate = method(tabs().getClass(), "d", Class.forName("bl"), int.class).invoke(tabs(), i);
            if (title.equals(method(candidate.getClass(), "a", String.class).invoke(candidate))) {
                clearDialogs(); method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), i); return;
            }
        }
        throw new AssertionError("Missing native browser tab " + title);
    }

    static void hailam() throws Exception {
        if (V4ContactIntegration.contact("bob") != null) throw new AssertionError("Use fresh dedicated DB: Hailam must not already have Bob");
        V4ContactIntegration.friends("before-friend-search"); command("bw");
        selectMenu(label(396), "search-new-friends-menu");
        int tx = request(922, 923, () -> command("bD")); waitPage("Search New Friends");
        String route = textField(packet("TX", 922), 2);
        if (!route.contains("c=profile&a=search&v=midlet")) throw new AssertionError("Original search route changed: " + route);
        save("friend-search-form");
        String all = postForm("Search New Friends", "friend-search-empty");
        if (!all.contains("mig33:privateChat(bob)") || all.contains("mig33:privateChat(hailam)")) throw new AssertionError("Empty friend results missing Bob or include viewer");
        until("friend results parsed", () -> profileLink("bob") != null);
        setText("query", "BOB"); String results = postForm("Search New Friends", "friend-search-bob");
        if (!results.contains("mig33:inviteFriend(bob)") || !results.contains("mig33:privateChat(bob)") || results.contains("mig33:privateChat(alice)"))
            throw new AssertionError("Wrong case-insensitive friend search results: " + results);
        until("searched Bob profile link", () -> profileLink("bob") != null);
        focusLink(profileLink("bob"), "friend-search-profile-link");
        tx = request(922, 923, () -> fire()); waitPage("User Profile");
        String html = responseText(923, tx, (short) 2);
        if (!html.contains("Presence: Offline") || !html.contains("Username: bob") || !html.contains("mig33:privateChat(bob)"))
            throw new AssertionError("Offline Bob profile is not complete: " + html);
        Files.writeString(out.resolve("offline-profile.html"), html); save("offline-profile");
        focusLink("mig33:privateChat(bob)", "offline-profile-chat-link"); fire();
        until("profile Chat opened native private tab", () -> (Integer) method(tabs().getClass(), "a", int.class, String.class).invoke(tabs(), "bob") >= 0);
        int tab = (Integer) method(tabs().getClass(), "a", int.class, String.class).invoke(tabs(), "bob");
        if ((Integer) method(tabs().getClass(), "i", int.class, int.class).invoke(tabs(), tab) != 3) throw new AssertionError("Profile Chat did not open private chat");
        save("offline-private-chat"); request(500, 1, () -> send("bob", queuedMessage));
        if (!"bob".equals(textField(packet("TX", 500), 4))) throw new AssertionError("Wrong offline DM recipient");
        selectBrowser("User Profile"); focusLink("mig33:inviteFriend(bob)", "profile-add-friend-link"); fire();
        until("native Add Contact form", () -> field(cc, "M", int.class).getInt(null) == 57 && get(canvas, "aj") != null);
        Object form = get(canvas, "aj"); int rows = (Integer) method(form.getClass(), "e", int.class).invoke(form);
        field(form.getClass(), "l", int.class).setInt(form, rows - 1); save("add-friend-native-form");
        request(405, 402, () -> command("bz"));
        until("native added Bob contact", () -> V4ContactIntegration.contact("bob") != null);
        V4ContactIntegration.friends("added-offline-friend");
        mark("queued-dm");
    }

    static String openLobbyInfo(String capture) throws Exception {
        clearDialogs(); selectRoom("Lobby"); command("bw"); selectMenu(label(358), capture + "-menu");
        int tx = request(922, 923, () -> command("bD")); waitPage("Room Info");
        String html = responseText(923, tx, (short) 2);
        Files.writeString(out.resolve(capture + ".html"), html); save(capture); return html;
    }

    static Map<String, String> moderationFields() throws Exception {
        if (page() == null) return Collections.emptyMap();
        for (Object candidate : (Vector) field(page().getClass(), "y", Vector.class).get(page())) {
            if (!candidate.getClass().getName().equals("s")) continue;
            Map<String, String> values = hidden(candidate);
            if ("bob".equals(values.get("target"))) return values;
        }
        return Collections.emptyMap();
    }

    static void pageModeration(String operation, String capture) throws Exception {
        until("native form for " + operation + " Bob", () -> operation.equals(moderationFields().get("operation")));
        Map<String, String> values = moderationFields();
        if (!values.containsKey("csrf") || !values.containsKey("binding") || !"4c6f626279".equalsIgnoreCase(values.get("room")))
            throw new AssertionError("Wrong native moderation hidden controls: " + values.keySet());
        String result = postForm("Room Info", capture);
        String next = operation.equals("mute") ? "unmute" : "mute";
        if (!result.contains("value=\"" + next + "\"")) throw new AssertionError("Room management did not change operation: " + result);
        until("native management form updated after " + operation, () -> next.equals(moderationFields().get("operation")));
        save(capture + "-updated");
    }

    static void participantRoles(String capture, int bobRole) throws Exception {
        V4RoomModerationIntegration.roles(capture, "alice:1", "bob:" + bobRole);
    }

    static void nativeUnmuteBob() throws Exception {
        clearDialogs(); selectRoom("Lobby"); command("bw"); selectMenu(label(32), "native-unmute-room-menu");
        int before = Connector.count("RX", 708); command("bD");
        until("native mute-state participant reply", () -> Connector.count("RX", 708) > before);
        until("native muted-user picker", () -> {
            Object picker = method(cc, "i", Class.forName("y")).invoke(canvas);
            return picker != null && field(picker.getClass(), "d", int.class).getInt(picker) == 45;
        });
        selectMenu("bob", "native-unmute-bob-picker");
        Object picker = method(cc, "i", Class.forName("y")).invoke(canvas);
        String suffix = (String) method(picker.getClass(), "d", String.class, int.class).invoke(picker, field(picker.getClass(), "m", int.class).getInt(picker));
        if (suffix == null || !suffix.contains(label(321))) throw new AssertionError("Muted Bob lacks native Un-Mute marker: " + suffix);
        request(710, 1, () -> command("bD"));
        if (!"Lobby".equals(textField(packet("TX", 710), 1)) || !"bob".equals(textField(packet("TX", 710), 2))) throw new AssertionError("Wrong native710 target");
        until("native unmute success dialog", () -> !((Vector) get(canvas, "aI")).isEmpty()); save("native-unmute-success"); clearDialogs();
    }

    static void alice() throws Exception {
        join("Lobby"); peer("bob", "joined"); peer("bob", "ordinary-info-checked");
        String html = openLobbyInfo("moderator-room-info");
        if (!html.contains("Mute bob")) throw new AssertionError("Moderator lacks visible mute control");
        pageModeration("mute", "page-mute-bob"); mark("muted");
        peer("bob", "muted-send-denied"); expectPrivate("DM while room-muted", "dm-remains-available-while-muted");
        participantRoles("participants-after-page-mute", 2);
        if (wireHasRoomText("Lobby", "MUTED page send must fail")) throw new AssertionError("Rejected room message delivered");
        openLobbyInfo("moderator-info-after-mute"); pageModeration("unmute", "page-unmute-bob"); mark("page-unmuted");
        expectRoom("Lobby", "Bob reply after page unmute", "reply-after-page-unmute"); peer("bob", "page-unmute-send-ok");
        openLobbyInfo("moderator-info-before-second-mute"); pageModeration("mute", "page-mute-for-native-unmute"); mark("muted-again");
        peer("bob", "second-muted-send-denied"); nativeUnmuteBob(); participantRoles("participants-after-native-unmute", 3); mark("native-unmuted");
        expectRoom("Lobby", "Bob reply after native unmute", "reply-after-native-unmute"); peer("bob", "native-unmute-send-ok");
        openLobbyInfo("moderator-info-final"); until("final native mute form restored", () -> "mute".equals(moderationFields().get("operation"))); save("moderator-info-final-ready");
        mark("finished"); peer("bob", "logged-out");
        Object contact = V4ContactIntegration.contact("bob"); int id = V4ContactIntegration.id(contact, "a");
        until("Bob offline presence drained before Alice logout", () -> {
            if (Connector.count("RX", 404) == 0) return false;
            Object p = packet("RX", 404);
            Integer who = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 1);
            Byte presence = (Byte) method(p.getClass(), "b", Byte.class, short.class).invoke(p, (short) 2);
            return who != null && who == id && presence != null && presence == 99;
        });
    }

    static void bob() throws Exception {
        expectPrivate(queuedMessage, "offline-dm-delivered-on-login");
        join("Lobby"); mark("joined"); peer("alice", "ready");
        String html = openLobbyInfo("ordinary-room-info");
        if (html.contains("<form") || html.contains("Mute bob")) throw new AssertionError("Ordinary Bob received moderator controls");
        mark("ordinary-info-checked"); peer("alice", "muted");
        sendRoom("Lobby", "MUTED page send must fail", 0); save("room-send-denied-after-page-mute"); clearDialogs(); mark("muted-send-denied");
        request(500, 1, () -> send("alice", "DM while room-muted"));
        peer("alice", "page-unmuted"); sendRoom("Lobby", "Bob reply after page unmute", 1); mark("page-unmute-send-ok");
        peer("alice", "muted-again"); sendRoom("Lobby", "MUTED second page send must fail", 0); save("room-send-denied-before-native-unmute"); clearDialogs(); mark("second-muted-send-denied");
        peer("alice", "native-unmuted"); sendRoom("Lobby", "Bob reply after native unmute", 1); mark("native-unmute-send-ok");
        peer("alice", "finished");
    }

    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; out = friendRoot.resolve(user); Connector.out = out; V4ContactIntegration.start();
            if (user.equals("hailam")) hailam(); else if (user.equals("alice")) alice(); else bob();
            if (Connector.count("TX", 417) != 0) throw new AssertionError("Room management incorrectly used personal block417");
            logout(); mark("logged-out");
            Files.writeString(out.resolve("result.txt"), "PASS original v4.62 friend discovery and moderation: " + user + " over " + transport + ".\n");
            System.out.println("PASS " + user + " friend discovery and moderation complete"); System.exit(0);
        } catch (Throwable error) {
            error.printStackTrace(); try { save("failure"); } catch (Throwable ignored) { }
            try { if (network != null && bc != null && field(bc, "c", boolean.class).getBoolean(null)) logout(); } catch (Throwable ignored) { }
            System.exit(1);
        }
    }
}
