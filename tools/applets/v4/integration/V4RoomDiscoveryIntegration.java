import java.nio.file.*;
import java.util.*;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.Canvas;

/** Original room menus, browser forms, command links and favorite categories. */
public class V4RoomDiscoveryIntegration extends V4VoteReportIntegration {
    static final Path discoveryRoot = Paths.get("data/applets/integration/room-discovery-out");
    static final String description = "Local friends & chat + 50% fun = yes";
    static String roomName, phase;

    static Object catalogModel() throws Exception { return get(canvas, "ad"); }
    static Vector catalogEntries() throws Exception { return (Vector) field(catalogModel().getClass(), "f", Vector.class).get(catalogModel()); }

    static void roomsTab() throws Exception {
        clearDialogs();
        int tab = (Integer) method(tabs().getClass(), "b", int.class, int.class).invoke(tabs(), 8);
        if (tab < 0) throw new AssertionError("Missing original Chat Rooms tab");
        method(tabs().getClass(), "g", void.class, int.class).invoke(tabs(), tab);
    }

    static void openCatalogMenu(int labelID, String title, String capture) throws Exception {
        roomsTab(); command("bw"); selectMenu(label(labelID), capture + "-menu");
        request(922, 923, () -> command("bD")); waitPage(title); save(capture);
    }

    static void waitPage(String title) throws Exception {
        until("native browser parsed " + title, () -> page() != null && title.equals(method(browser().getClass(), "a", String.class).invoke(browser())));
    }

    static Object textControl(String name) throws Exception {
        if (page() == null) return null;
        for (Object control : (Vector) field(page().getClass(), "m", Vector.class).get(page())) {
            int kind = field(control.getClass(), "b", int.class).getInt(control);
            if (kind != 4 && kind != 8) continue;
            Object value = field(control.getClass(), kind == 4 ? "i" : "a", Class.forName("bj")).get(control);
            if (name.equals(field(value.getClass(), kind == 4 ? "c" : "a", String.class).get(value))) return control;
        }
        return null;
    }

    static void setText(String name, String text) throws Exception {
        until("native form field " + name + " parsed", () -> textControl(name) != null);
        Object control = textControl(name);
        int kind = field(control.getClass(), "b", int.class).getInt(control);
        Object value = field(control.getClass(), kind == 4 ? "i" : "a", Class.forName("bj")).get(control);
        field(value.getClass(), kind == 4 ? "b" : "c", String.class).set(value, text);
        if (kind == 8) {
            int width = field(value.getClass(), "b", int.class).getInt(value);
            if (width < 200) throw new AssertionError("Native description textarea too narrow: " + width);
        }
    }

    static String postForm(String title, String capture) throws Exception {
        Object p = page();
        Vector controls = (Vector) field(p.getClass(), "m", Vector.class).get(p);
        Vector focus = (Vector) field(p.getClass(), "n", Vector.class).get(p);
        int submit = -1;
        for (int i = 0; i < focus.size(); i++) {
            Object ref = focus.elementAt(i);
            Object control = controls.elementAt(field(ref.getClass(), "b", int.class).getInt(ref));
            // Search now has three independent filter forms; this helper submits
            // the first (name/description) form after setText("query", ...).
            if (field(control.getClass(), "b", int.class).getInt(control) == 2) { submit = i; break; }
        }
        if (submit < 0) throw new AssertionError("Missing native form submit control");
        field(p.getClass(), "r", int.class).setInt(p, submit);
        String[] form = (String[]) method(browser().getClass(), "f", String[].class).invoke(browser());
        if (form == null || !form[1].startsWith("http://127.0.0.1:8080/sites/index.php?") || form[1].contains("&amp;") || form[2] == null)
            throw new AssertionError("Wrong original serialized form contract: " + Arrays.toString(form));
        save(capture + "-submit");
        int tx = request(922, 923, () -> method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE)));
        if (!form[2].equals(textField(packet("TX", 922), 3))) throw new AssertionError("Original form body changed before transmission");
        Object response = packet("RX", 923);
        Integer status = (Integer) method(response.getClass(), "d", Integer.class, short.class).invoke(response, (short) 1);
        String html = responseText(923, tx, (short) 2);
        Files.writeString(out.resolve(capture + ".html"), html);
        if (status == null || status != 200) throw new AssertionError("Native form returned " + status + ": " + html);
        waitPage(title); save(capture + "-result");
        return html;
    }

    static int linkFocus(String href) throws Exception {
        if (page() == null) return -1;
        Vector focus = (Vector) field(page().getClass(), "n", Vector.class).get(page());
        for (int i = 0; i < focus.size(); i++) {
            Object ref = focus.elementAt(i);
            if (href.equals(field(ref.getClass(), "c", String.class).get(ref))) return i;
        }
        return -1;
    }

    static void joinPageLink(String capture) throws Exception {
        String href = "mig33:joinChatroom(" + roomName + ")";
        until("literal native Join Room link parsed", () -> linkFocus(href) >= 0);
        field(page().getClass(), "r", int.class).setInt(page(), linkFocus(href)); save(capture + "-link");
        int tx = request(703, 701, () -> method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE)));
        until("native join acknowledgment", () -> received(1, tx));
        until("native joined room ready", () -> roomChat(roomName) != null && field(roomChat(roomName).getClass(), "d", boolean.class).getBoolean(roomChat(roomName)));
        if (!roomName.equals(textField(packet("TX", 703), 1))) throw new AssertionError("Native link corrupted room name with spaces");
        selectRoom(roomName); save(capture + "-joined");
    }

    static void roomInfo(String capture) throws Exception {
        clearDialogs(); selectRoom(roomName); command("bw"); selectMenu(label(358), capture + "-menu");
        int tx = request(922, 923, () -> command("bD")); waitPage("Room Info");
        String html = responseText(923, tx, (short) 2);
        if (!html.contains(roomName) || !html.contains("hailam") || !html.contains("50") || !html.contains("Local friends &amp; chat + 50% fun = yes"))
            throw new AssertionError("Room info lacks persisted room details: " + html);
        Files.writeString(out.resolve(capture + ".html"), html); save(capture);
    }

    static Map<String, Integer> categoryRows(int category) throws Exception {
        Map<String, Integer> names = new TreeMap<>();
        synchronized (catalogEntries()) {
            for (Object row : catalogEntries()) {
                if (field(row.getClass(), "c", int.class).getInt(row) == 2 && field(row.getClass(), "k", short.class).getShort(row) == category) {
                    String name = (String) field(row.getClass(), "c", String.class).get(row);
                    names.put(name, names.getOrDefault(name, 0) + 1);
                    if (category == 2 && !field(row.getClass(), "c", boolean.class).getBoolean(row)) throw new AssertionError("Favorite row lacks original favorite menu flag");
                }
            }
        }
        return names;
    }

    static void loadCategory(int category, String capture) throws Exception {
        roomsTab(); Object model = catalogModel(); boolean found = false;
        synchronized (catalogEntries()) {
            for (Object row : catalogEntries()) if (field(row.getClass(), "c", int.class).getInt(row) == 1) {
                int id = field(row.getClass(), "k", short.class).getShort(row);
                method(model.getClass(), "a", void.class, boolean.class, model.getClass()).invoke(model, id != category, row);
                if (id == category) found = true;
            }
        }
        if (!found) throw new AssertionError("Missing original room category " + category);
        request(716, 717, () -> method(cc, "q", void.class).invoke(canvas));
        until("native category loading completed", () -> !field(model.getClass(), "b", boolean.class).getBoolean(method(model.getClass(), "b", model.getClass(), int.class).invoke(model, category)));
        save(capture); Files.writeString(out.resolve(capture + "-model.txt"), categoryRows(category).toString() + "\n");
    }

    static void favoriteRoom(int response, String capture) throws Exception {
        clearDialogs(); selectRoom(roomName); command("bw"); selectMenu(label(267), capture + "-menu");
        request(711, response, () -> command("bD"));
        until("native pending favorite request completed", () -> !(Boolean) method(bc, "b", boolean.class, short.class).invoke(network, (short) 711));
        if (!roomName.equals(textField(packet("TX", 711), 1))) throw new AssertionError("Wrong native favorite room name");
        save(capture); clearDialogs();
    }

    static void refreshCategoryFooter(int category, String capture) throws Exception {
        roomsTab(); Object model = catalogModel(); Map<String, Integer> before = categoryRows(category);
        if (before.isEmpty() || before.values().stream().anyMatch(count -> count != 1))
            throw new AssertionError("Expected unique nonempty category before footer refresh: " + before);
        int selected = -1; Vector rows = catalogEntries();
        for (int i = 0; i < rows.size(); i++) {
            Object row = rows.elementAt(i);
            if (field(row.getClass(), "c", int.class).getInt(row) == 3 && field(row.getClass(), "k", short.class).getShort(row) == category)
                selected = i;
        }
        if (selected < 0) throw new AssertionError("Missing native category refresh footer " + category);
        field(model.getClass(), "g", int.class).setInt(model, selected);
        request(716, 717, () -> method(cc, "keyPressed", void.class, int.class).invoke(canvas, ((Canvas) canvas).getKeyCode(Canvas.FIRE)));
        Object sent = packet("TX", 716);
        Short sentCategory = (Short) method(sent.getClass(), "c", Short.class, short.class).invoke(sent, (short) 1);
        Byte flag = (Byte) method(sent.getClass(), "b", Byte.class, short.class).invoke(sent, (short) 2);
        if (sentCategory == null || sentCategory != category || flag == null || flag != 0)
            throw new AssertionError("Footer did not emit expected category request with flag 0");
        until("native footer refresh completed", () -> !field(model.getClass(), "b", boolean.class).getBoolean(method(model.getClass(), "b", model.getClass(), int.class).invoke(model, category)));
        Map<String, Integer> after = categoryRows(category);
        if (!before.equals(after)) throw new AssertionError("Native footer refresh duplicated or changed catalog: " + before + " -> " + after);
        save(capture); Files.writeString(out.resolve(capture + "-model.txt"), after.toString() + "\n");
    }

    static void removeFavorite() throws Exception {
        roomsTab(); Object model = catalogModel(); int selected = -1;
        Vector rows = catalogEntries();
        for (int i = 0; i < rows.size(); i++) {
            Object row = rows.elementAt(i);
            if (field(row.getClass(), "c", int.class).getInt(row) == 2 && field(row.getClass(), "k", short.class).getShort(row) == 2 && roomName.equals(field(row.getClass(), "c", String.class).get(row))) selected = i;
        }
        if (selected < 0) throw new AssertionError("No saved room to remove");
        field(model.getClass(), "g", int.class).setInt(model, selected);
        command("bw"); selectMenu(label(256), "remove-favorite-menu"); request(712, 1, () -> command("bD"));
        Object sent = packet("TX", 712);
        Short category = (Short) method(sent.getClass(), "c", Short.class, short.class).invoke(sent, (short) 1);
        if (category == null || category != 2 || !roomName.equals(textField(sent, 2))) throw new AssertionError("Wrong native remove-favorite packet");
        until("native favorite row removed", () -> !categoryRows(2).containsKey(roomName)); save("favorite-removed");
        loadCategory(2, "favorites-empty-after-refresh");
        if (categoryRows(2).containsKey(roomName)) throw new AssertionError("Removed favorite returned after refresh");
    }

    static void search(String capture) throws Exception {
        openCatalogMenu(270, "Search Rooms", capture + "-form");
        String all = postForm("Search Rooms", "search-empty-query");
        if (!all.contains(roomName) || !all.contains("Lobby")) throw new AssertionError("Empty native query did not browse available rooms: " + all);
        until("empty-query results rendered", () -> linkFocus("mig33:joinChatroom(" + roomName + ")") >= 0);
        setText("query", roomName);
        String html = postForm("Search Rooms", capture);
        if (!html.contains(roomName) || !html.contains("mig33:joinChatroom(" + roomName + ")")) throw new AssertionError("Search did not find created room: " + html);
        until("search result Join Room rendered", () -> linkFocus("mig33:joinChatroom(" + roomName + ")") >= 0);
        save(capture + "-rendered");
    }

    static void creator() throws Exception {
        openCatalogMenu(91, "Create Room", "create-name-form"); setText("name", roomName);
        postForm("Create Room", "create-name");
        setText("description", description); postForm("Room Created", "create-details");
        joinPageLink("created-room"); roomInfo("created-room-info");
        favoriteRoom(701, "favorite-first-add"); loadCategory(2, "favorites-after-add");
        if (!Integer.valueOf(1).equals(categoryRows(2).get(roomName))) throw new AssertionError("Saved room missing or duplicated");
        favoriteRoom(1, "favorite-repeat-ack");
        if (!Integer.valueOf(1).equals(categoryRows(2).get(roomName))) throw new AssertionError("Repeated add inserted duplicate native row");
        refreshCategoryFooter(2, "favorites-footer-refresh");
        loadCategory(1, "global-catalog-after-create");
        if (!categoryRows(1).containsKey(roomName)) throw new AssertionError("Created room not in global native catalog");
        refreshCategoryFooter(1, "global-footer-refresh");
        refreshCategoryFooter(1, "global-footer-repeat");
    }

    static void observer() throws Exception {
        loadCategory(2, "other-account-favorites");
        if (categoryRows(2).containsKey(roomName)) throw new AssertionError("Another user's favorite leaked");
        search("search-created-room"); joinPageLink("search-result"); roomInfo("observer-room-info");
        roomParticipants(roomName, "observer-participants", "bob");
    }

    static void returning() throws Exception {
        loadCategory(2, "favorites-after-new-login");
        if (!Integer.valueOf(1).equals(categoryRows(2).get(roomName))) throw new AssertionError("Favorite lost across independent JVM logins");
        removeFavorite(); loadCategory(1, "global-room-survives-unfavorite");
        if (!categoryRows(1).containsKey(roomName)) throw new AssertionError("Removing favorite deleted public room");
    }

    public static void main(String[] args) {
        try {
            user = args[0]; transport = args[1]; phase = System.getenv("ROOM_DISCOVERY_PHASE");
            roomName = Files.readString(discoveryRoot.resolve("room-name.txt")).trim();
            out = discoveryRoot.resolve(phase); Connector.out = out; V4ContactIntegration.start();
            if ("creator".equals(phase)) creator(); else if ("observer".equals(phase)) observer(); else returning();
            logout(); Files.writeString(out.resolve("result.txt"), "PASS original v4.62 room discovery: " + phase + " / " + user + " over " + transport + ".\n");
            System.out.println("PASS " + phase + " room discovery complete"); System.exit(0);
        } catch (Throwable error) {
            error.printStackTrace(); try { save("failure"); } catch (Throwable ignored) { }
            try { if (network != null && bc != null && field(bc, "c", boolean.class).getBoolean(null)) logout(); } catch (Throwable ignored) { }
            System.exit(1);
        }
    }
}
