import java.io.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import java.awt.image.BufferedImage;
import javax.imageio.ImageIO;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.*;

/** Executes original 4.62 page parser/forms, palette UI and recipient renderer. */
public class V4PagesIntegration extends V4SlashIntegration {
    static final Path pagesRoot = Paths.get("data/applets/integration/pages-out");
    static final String base = "http://127.0.0.1:8080";
    static final String punctuation = "Native & plus + percent 50% equals = café 世界";
    static final String coloured = "Original colour picker red message";
    static Object web;
    static String communityID, ownedRoom;
    static int opened, posted;

    static void peerPage(String who, String marker) throws Exception {
        long end = System.currentTimeMillis() + 300000;
        while (System.currentTimeMillis() < end) {
            if (Files.exists(pagesRoot.resolve(who).resolve(marker))) return;
            if (Files.exists(pagesRoot.resolve(who).resolve("failed"))) throw new AssertionError(who + " failed; see its runtime.log");
            Thread.sleep(100);
        }
        throw new AssertionError("Timeout waiting for " + who + ": " + marker);
    }

    static void firePage() throws Exception {
        method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE));
    }

    static String parsedText() throws Exception {
        if (page() == null) return "";
        return ((Vector) field(page().getClass(), "b", Vector.class).get(page())).toString();
    }

    // Original k.g defers tab-body painting for 400ms (k.c); a parsed page can
    // therefore still paint only gray chrome. Paint also clears c when its
    // original worker is waiting (k.d), so call original paint before checking
    // readiness; then require two stable populated captures without altering it.
    static void savePage(String capture) throws Exception {
        Object expectedPage = page();
        long deadline = System.currentTimeMillis() + 20000;
        long previousHash = 0;
        boolean previousPopulated = false;
        int attempts = 0;
        while (System.currentTimeMillis() < deadline) {
            if (page() == expectedPage &&
                field(cc, "M", int.class).getInt(null) == 48) {
                V4Integration.save(capture);
                attempts++;
                BufferedImage rendered = ImageIO.read(out.resolve(capture + ".png").toFile());
                Map<Integer, Integer> pixels = new HashMap<>();
                long hash = 1125899906842597L;
                // Exclude the tab bar, borders and scrollbar; inspect content.
                for (int y = 24; y < 306; y++) for (int x = 4; x < 232; x++) {
                    int rgb = rendered.getRGB(x, y) & 0xffffff;
                    pixels.put(rgb, pixels.getOrDefault(rgb, 0) + 1);
                    hash = hash * 31 + rgb;
                }
                double dominant = (double) Collections.max(pixels.values()) / (228 * 282);
                boolean populated = dominant < 0.995 && pixels.size() >= 3 &&
                    !field(tabs().getClass(), "c", boolean.class).getBoolean(tabs());
                if (populated && previousPopulated && hash == previousHash) {
                    Files.writeString(out.resolve("page-render-metrics.tsv"), capture + "\t" + attempts + "\t" +
                        pixels.size() + "\t" + String.format(Locale.ROOT, "%.8f", dominant) + "\t" + hash + "\n",
                        StandardOpenOption.CREATE, StandardOpenOption.APPEND);
                    System.out.println("PASS stable original Canvas body " + capture + " colors=" + pixels.size());
                    return;
                }
                previousPopulated = populated;
                previousHash = hash;
            }
            Thread.sleep(80);
        }
        throw new AssertionError("No stable populated original Canvas body for " + capture +
            "; paint attempts=" + attempts + "; state=" + field(cc, "M", int.class).getInt(null) +
            "; deferred tab=" + field(tabs().getClass(), "c", boolean.class).getBoolean(tabs()));
    }

    static String acceptPage(int tx, String capture) throws Exception {
        String html = responseText(923, tx, (short) 2);
        Files.writeString(out.resolve(capture + ".html"), html);
        Object response = packet("RX", 923);
        Integer status = (Integer) method(response.getClass(), "d", Integer.class, short.class).invoke(response, (short) 1);
        if (status == null || status != 200) throw new AssertionError(capture + " returned " + status + ": " + html);
        Matcher titleMatch = Pattern.compile("<title>(.*?)</title>", Pattern.DOTALL).matcher(html);
        if (!titleMatch.find() || titleMatch.group(1).isEmpty()) throw new AssertionError(capture + " missing title");
        String title = titleMatch.group(1);
        Vector expected = (Vector) method(Class.forName("y"), "a", Vector.class, InputStreamReader.class).invoke(null,
            new InputStreamReader(new ByteArrayInputStream(html.getBytes(StandardCharsets.UTF_8)), StandardCharsets.UTF_8));
        until("original HTML parsed/rendered " + capture, () -> page() != null &&
            expected.equals(field(page().getClass(), "b", Vector.class).get(page())) &&
            field(page().getClass(), "d", boolean.class).getBoolean(page()) &&
            title.equals(method(browser().getClass(), "a", String.class).invoke(browser())));
        Vector controls = (Vector) field(page().getClass(), "m", Vector.class).get(page());
        if (controls.isEmpty()) throw new AssertionError(capture + " has no original display elements");
        savePage(capture);
        Files.writeString(out.resolve(capture + "-model.txt"), "Title: " + title + "\nDisplay elements: " + controls.size() +
            "\nFocus entries: " + ((Vector) field(page().getClass(), "n", Vector.class).get(page())).size() + "\n" + parsedText() + "\n");
        web = browser();
        return html;
    }

    static String open(String url, String capture) throws Exception {
        clearDialogs();
        String absolute = url.startsWith("http") ? url : base + url;
        int tx = request(922, 923, () -> method(cc, "a", void.class, String.class, String.class, boolean.class, boolean.class, Class.forName("bl"))
            .invoke(canvas, absolute, null, true, true, web));
        String html = acceptPage(tx, capture); opened++;
        Files.writeString(out.resolve("pages.tsv"), capture + "\t" + absolute + "\t" + tx + "\t200\n", StandardOpenOption.CREATE, StandardOpenOption.APPEND);
        return html;
    }

    static int submitFocus(String label) throws Exception {
        Vector controls = (Vector) field(page().getClass(), "m", Vector.class).get(page());
        Vector focus = (Vector) field(page().getClass(), "n", Vector.class).get(page());
        for (int i = 0; i < focus.size(); i++) {
            Object ref = focus.elementAt(i), control = controls.elementAt(field(ref.getClass(), "b", int.class).getInt(ref));
            if (field(control.getClass(), "b", int.class).getInt(control) != 2) continue;
            Object button = field(control.getClass(), "j", Class.forName("bj")).get(control);
            if (label.equals(field(button.getClass(), "b", String.class).get(button))) return i;
        }
        return -1;
    }

    static String post(String button, String capture) throws Exception {
        until("native submit " + button, () -> submitFocus(button) >= 0);
        field(page().getClass(), "r", int.class).setInt(page(), submitFocus(button));
        String[] form = (String[]) method(browser().getClass(), "f", String[].class).invoke(browser());
        if (form == null || form[2] == null || !form[1].startsWith(base + "/sites/index.php?") || form[1].contains("&amp;"))
            throw new AssertionError("Invalid native form contract: " + Arrays.toString(form));
        savePage(capture + "-before");
        int tx = request(922, 923, () -> firePage());
        if (!form[1].equals(textField(packet("TX", 922), 2)) || !form[2].equals(textField(packet("TX", 922), 3)))
            throw new AssertionError("Native serialized POST changed in transport");
        Files.writeString(out.resolve(capture + "-submitted.txt"), form[1] + "\n" + form[2] + "\n");
        posted++;
        return acceptPage(tx, capture);
    }

    // Several independent room-setting forms share the final field name.
    // Pick the nearest preceding control belonging to the chosen submit button.
    static void setFormText(String button, String name, String text) throws Exception {
        int focusIndex = submitFocus(button);
        if (focusIndex < 0) throw new AssertionError("No native form " + button);
        Vector focus = (Vector) field(page().getClass(), "n", Vector.class).get(page());
        Object ref = focus.elementAt(focusIndex);
        int end = field(ref.getClass(), "b", int.class).getInt(ref);
        Vector controls = (Vector) field(page().getClass(), "m", Vector.class).get(page());
        for (int i = end - 1; i >= 0; i--) {
            Object control = controls.elementAt(i);
            int kind = field(control.getClass(), "b", int.class).getInt(control);
            if (kind != 4 && kind != 8) continue;
            Object value = field(control.getClass(), kind == 4 ? "i" : "a", Class.forName("bj")).get(control);
            if (!name.equals(field(value.getClass(), kind == 4 ? "c" : "a", String.class).get(value))) continue;
            field(value.getClass(), kind == 4 ? "b" : "c", String.class).set(value, text);
            return;
        }
        throw new AssertionError("No field " + name + " before " + button);
    }

    static void checkContains(String html, String expected, String what) {
        if (!html.contains(expected)) throw new AssertionError(what + " missing " + expected + ": " + html);
    }

    static String url(String c, String a) { return "/sites/index.php?c=" + c + "&a=" + a + "&v=midlet"; }

    static void createFixtures() throws Exception {
        ownedRoom = Files.readString(pagesRoot.resolve("room-name.txt")).trim();
        open(url("chatroom", "create"), "fixture-room-form"); setText("name", ownedRoom);
        post("Continue", "fixture-room-name");
        setText("description", punctuation); post("Create Room", "fixture-room-created");
        join(ownedRoom);
        open(url("chatroom", "setup") + "&manage=1&roomName=" + ownedRoom.replace(" ", "%20"), "room-settings");
        setFormText("Save Capacity (2 to 100)", "body", "70");
        String settings = post("Save Capacity (2 to 100)", "room-capacity-saved");
        checkContains(settings, "Capacity: 70", "Room capacity");
        setFormText("Save Keywords", "body", "native original templ");
        settings = post("Save Keywords", "room-keywords-saved");
        checkContains(settings, "Keywords: native original templ", "Room keywords");
        setFormText("Save Allow user vote-kicking (1=yes, 0=no)", "body", "0");
        settings = post("Save Allow user vote-kicking (1=yes, 0=no)", "room-user-kicks-disabled");
        checkContains(settings, "User vote-kicking enabled: false", "Room kicking policy");
        open(url("group", "list_my_groups"), "community-create-form");
        setText("name", Files.readString(pagesRoot.resolve("community-name.txt")).trim());
        post("Create Public Group", "community-name");
        setText("description", punctuation);
        String html = post("Create Group", "community-created");
        Matcher id = Pattern.compile("(?:cid|group_id)=([0-9]+)").matcher(html);
        if (!id.find()) throw new AssertionError("No native created community link: " + html);
        communityID = id.group(1);
        open(url("group", "view_forum") + "&cid=" + communityID, "community-forum-form");
        setText("body", punctuation); html = post("Post", "community-forum-post");
        checkContains(html, "Native &amp; plus + percent 50% equals = café 世界", "Forum punctuation");
        clearDialogs(); privateTab("bob");
        int tx = request(751, 750, () -> method(bc, "a", void.class, String.class, String.class, String.class, byte.class)
            .invoke(network, "bob", "carol", null, (byte) 1));
        group = responseText(750, tx, (short) 1);
        until("original private group created", () -> groupTab() >= 0);
    }

    static void allRoutes() throws Exception {
        for (String line : Files.readAllLines(pagesRoot.resolve("routes.tsv"))) {
            String[] fields = line.split("\t", 3);
            String route = fields[1].replace("{ROOM}", ownedRoom.replace(" ", "%20"))
                .replace("{COMMUNITY}", communityID).replace("{CHATGROUP}", group);
            String html = open(route, fields[0]);
            if (html.contains("This page has not been implemented") || html.contains("Unsupported feature page"))
                throw new AssertionError("Generic unimplemented page: " + fields[0]);
        }
        mark("routes-complete");
    }

    static void serviceForms() throws Exception {
        peerPage("bob", "updates-subscribed");
        open(url("buzz", "home"), "buzz-form"); setText("body", punctuation);
        String html = post("Publish", "buzz-published");
        checkContains(html, "Native &amp; plus + percent 50% equals = café 世界", "Buzz punctuation");
        mark("updates-published"); peerPage("bob", "updates-received");
        open(url("email", "compose") + "&to=bob", "mail-compose"); setText("body", punctuation);
        html = post("Save Request", "mail-sent"); checkContains(html, "local_delivered", "Local mail delivery"); mark("mail-sent");
        open(url("account", "recharge_credit"), "credit-recharge-form");
        html = post("Add 10.00 test credits", "credit-recharged"); checkContains(html, "Added 10.00", "Recharge receipt");
        open(url("account", "transfer_credit") + "&recipient_username=bob", "credit-transfer-form");
        setText("amount", "1.25"); html = post("Transfer", "credit-transferred"); checkContains(html, "Transferred 1.25", "Transfer receipt");
        open(url("store", "home") + "&ty=1&username=bob", "gift-store");
        html = post("Buy Rose", "gift-purchased"); checkContains(html, "Rose delivered to bob", "Gift receipt"); mark("gift-sent");
        open(url("store", "home") + "&ty=5", "theme-store");
        post("Buy Ocean blue", "theme-purchased");
        int before = Connector.count("RX", 1000);
        html = post("Use Ocean blue", "theme-applied"); checkContains(html, "Theme selected", "Theme selection");
        until("whole native theme1000 received", () -> Connector.count("RX", 1000) > before);
        Object theme = field(cc, "j", Class.forName("ay")).get(null);
        Integer background = (Integer) method(theme.getClass(), "d", Integer.class, short.class).invoke(theme, (short) 139);
        if (background == null || background == 0xffffff) throw new AssertionError("Theme selection did not alter original chat background");
        selectConversation(1, "bob"); save("native-theme-chat-background");
        Files.writeString(out.resolve("theme-model.txt"), "Theme139=" + background + "\nFields=" + method(theme.getClass(), "a", int.class).invoke(theme) + "\n");
        mark("services-complete");
    }

    static void nativeColour() throws Exception {
        until("native palette924 available", () -> Connector.count("RX", 924) > 0 && ((Vector) get(canvas, "aM")).size() > 1);
        selectConversation(1, "bob"); command("bw"); selectMenu(label(342), "colour-write-menu"); command("bD");
        TextBox editor = (TextBox) field(cc, "ce", TextBox.class).get(canvas);
        Display display = (Display) field(cc, "ae", Display.class).get(canvas);
        until("original composer current", () -> display.getCurrent() == editor);
        Command colourCommand = (Command) field(cc, "bN", Command.class).get(canvas);
        if (!"Change Text Color".equals(colourCommand.getLabel())) throw new AssertionError("Wrong original palette command");
        method(cc, "commandAction", void.class, Command.class, Displayable.class).invoke(canvas, colourCommand, editor);
        until("native palette state59", () -> field(cc, "M", int.class).getInt(null) == 59 && display.getCurrent() == canvas);
        Object picker = get(canvas, "bp");
        for (int i = 0; i < 30 && ((Integer) method(picker.getClass(), "q", int.class).invoke(picker)) != 0xff0000; i++)
            method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.RIGHT));
        int chosen = (Integer) method(picker.getClass(), "q", int.class).invoke(picker);
        if (chosen != 0xff0000) throw new AssertionError("Original picker failed to select red: " + chosen);
        save("native-colour-picker-red"); command("bD");
        until("colour selection returned to composer", () -> display.getCurrent() == editor);
        if (field(conversation(1, "bob").getClass(), "e", int.class).getInt(conversation(1, "bob")) != chosen)
            throw new AssertionError("Picker did not update original conversation");
        editor.setString(coloured);
        request(500, 1, () -> method(cc, "commandAction", void.class, Command.class, Displayable.class)
            .invoke(canvas, field(cc, "bA", Command.class).get(canvas), editor));
        Integer sent = (Integer) method(packet("TX", 500).getClass(), "d", Integer.class, short.class).invoke(packet("TX", 500), (short) 13);
        if (sent == null || sent != chosen) throw new AssertionError("Native500 omitted selected RGB");
        until("composer closed", () -> display.getCurrent() == canvas); selectConversation(1, "bob"); save("native-colour-author-echo");
        mark("colour-sent"); peerPage("bob", "colour-received");
    }

    static void alicePages() throws Exception {
        peerPage("bob", "ready"); nativeColour(); createFixtures(); allRoutes(); serviceForms();
        mark("finished"); peerPage("bob", "logged-out");
        // Drain the other client's offline notification before original logout.
        Thread.sleep(800); logout();
    }

    static void bobPages() throws Exception {
        peerPage("alice", "colour-sent");
        until("recipient received coloured500", () -> !incoming(1, coloured).isEmpty());
        Object p = incoming(1, coloured).get(0);
        Integer rgb = (Integer) method(p.getClass(), "d", Integer.class, short.class).invoke(p, (short) 13);
        if (rgb == null || rgb != 0xff0000) throw new AssertionError("Recipient colour differs");
        until("recipient original coloured message rendered", () -> conversation(1, "alice") != null && visibleRows(1, "alice").contains(coloured));
        selectConversation(1, "alice"); save("native-colour-recipient");
        String model = ((Vector) field(conversation(1, "alice").getClass(), "y", Vector.class).get(conversation(1, "alice"))).toString();
        if (!model.contains("16711680")) throw new AssertionError("Original wrapped rows lack red colour marker");
        Files.writeString(out.resolve("recipient-colour-model.txt"), model + "\n"); mark("colour-received");
        int updateTab = (Integer) method(tabs().getClass(), "b", int.class, int.class).invoke(tabs(), 2);
        if (updateTab < 0) {
            request(921, 1, () -> method(cc, "ar", void.class).invoke(canvas));
        } else {
            request(921, 1, () -> method(bc, "a", void.class, byte.class).invoke(network, (byte) 1));
            method(cc, "ar", void.class).invoke(canvas);
        }
        // The subscription ACK and initial backlog share a transport response.
        // Drain them before counting the separately published live notification.
        Thread.sleep(800);
        int oldUpdates = Connector.count("RX", 422);
        mark("updates-subscribed"); peerPage("alice", "updates-published");
        until("live native activity422", () -> Connector.count("RX", 422) > oldUpdates);
        String summary = textField(packet("RX", 422), 4);
        checkContains(summary, "alice posted an update.", "Live activity summary");
        checkContains(summary, "<a href=", "Live activity link");
        until("original activity renderer populated", () -> activityText().contains("alice posted an update."));
        method(cc, "ar", void.class).invoke(canvas); save("native-updates-live-buzz");
        Files.writeString(out.resolve("native-updates-model.txt"), summary + "\n" + activityText() + "\n");
        request(921, 1, () -> method(bc, "a", void.class, byte.class).invoke(network, (byte) 0));
        mark("updates-received");
        peerPage("alice", "services-complete");
        String html = open(url("email", "compose"), "recipient-mail-inbox");
        checkContains(html, "Native &amp; plus + percent 50% equals = café 世界", "Recipient local inbox");
        html = open(url("store", "home") + "&ty=1", "recipient-gift-store"); checkContains(html, "Rose from alice", "Recipient owned gift");
        html = open(url("account", "home"), "recipient-account"); checkContains(html, "1.25", "Recipient transferred credits");
        peerPage("alice", "finished"); logout(); mark("logged-out");
    }

    static String activityText() throws Exception {
        Object feed = get(canvas, "ac");
        Vector entries = (Vector) field(feed.getClass(), "e", Vector.class).get(feed);
        StringBuilder text = new StringBuilder();
        for (Object entry : (Vector) entries.clone()) {
            Vector rows = (Vector) field(entry.getClass(), "f", Vector.class).get(entry);
            for (Object row : rows) text.append(field(row.getClass(), "a", String.class).get(row)).append(' ');
            text.append('\n');
        }
        return text.toString();
    }

    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; out = pagesRoot.resolve(user); Connector.out = out;
            V4ContactIntegration.start();
            if (user.equals("alice")) alicePages(); else bobPages();
            Files.writeString(out.resolve("result.txt"), "PASS original v4.62 pages and colours: " + user + " / " + transport +
                "; rendered pages=" + opened + "; native POST forms=" + posted + ".\n");
            System.exit(0);
        } catch (Throwable error) {
            error.printStackTrace(); try { mark("failed"); save("failure"); } catch (Throwable ignored) { }
            try { if (network != null && bc != null && field(bc, "c", boolean.class).getBoolean(null)) logout(); } catch (Throwable ignored) { }
            System.exit(1);
        }
    }
}
