import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import javax.microedition.io.Connector;

/**
 * Interactive original-client peer for the mobile protocol acceptance test.
 *
 * The unchanged 4.62 MIDlet performs login, roster synchronization, transport
 * scheduling, serialization, reply dispatch and chat-model construction.
 * This harness only selects configuration and invokes original UI builders.
 *
 * Standard output is NDJSON. Original emulator/Connector diagnostics go to
 * standard error. Input is one command per line with tab-separated base64 UTF-8
 * arguments: EXPECT peer text, SEND peer text, PRESENCE peer value;
 * ROOM_JOIN room, ROOM_SEND room text, ROOM_EXPECT room sender text,
 * ROOM_PARTICIPANTS room semicolon-separated-names, ROOM_LEAVE room;
 * or STOP (without arguments).
 */
public final class V4MobilePeer extends V4RoomIntegration {
    private static final PrintStream replies = System.out;
    private static int roomCapture;

    private static String quote(String value) {
        StringBuilder result = new StringBuilder("\"");
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            switch (c) {
                case '"': result.append("\\\""); break;
                case '\\': result.append("\\\\"); break;
                case '\b': result.append("\\b"); break;
                case '\f': result.append("\\f"); break;
                case '\n': result.append("\\n"); break;
                case '\r': result.append("\\r"); break;
                case '\t': result.append("\\t"); break;
                default:
                    if (c < 32) result.append(String.format("\\u%04x", (int) c));
                    else result.append(c);
            }
        }
        return result.append('"').toString();
    }

    private static synchronized void emit(String event, String... fields) {
        StringBuilder json = new StringBuilder("{\"event\":").append(quote(event));
        for (int i = 0; i < fields.length; i += 2) {
            json.append(',').append(quote(fields[i])).append(':').append(quote(fields[i + 1]));
        }
        replies.println(json.append('}'));
        replies.flush();
    }

    private static String argument(String encoded) {
        return new String(Base64.getDecoder().decode(encoded), StandardCharsets.UTF_8);
    }

    /** Parse captured bytes with original ay, then require original UI dispatch. */
    private static boolean receivedMessage(String sender, String text) throws Exception {
        return receivedMessage(sender, text, null);
    }

    private static boolean receivedMessage(String sender, String text, String room) throws Exception {
        List<String> files = new ArrayList<>();
        synchronized (Connector.events) {
            for (String event : Connector.events) {
                String[] columns = event.split("\t");
                if (columns.length >= 5 && columns[0].equals("RX") && columns[1].equals("500")) {
                    files.add(columns[4]);
                }
            }
        }
        Class<?> packetClass = Class.forName("ay");
        Class<?> arrayClass = java.lang.reflect.Array.newInstance(packetClass, 0).getClass();
        for (String filename : files) {
            Object[] packets = (Object[]) method(packetClass, "a", arrayClass, byte[].class)
                .invoke(null, Files.readAllBytes(out.resolve(filename)));
            for (Object packet : packets) {
                String from = (String) method(packetClass, "f", String.class, short.class)
                    .invoke(packet, (short) 2);
                String body = (String) method(packetClass, "f", String.class, short.class)
                    .invoke(packet, (short) 8);
                Byte kind = (Byte) method(packetClass, "b", Byte.class, short.class)
                    .invoke(packet, (short) 3);
                String target = (String) method(packetClass, "f", String.class, short.class)
                    .invoke(packet, (short) 4);
                if (!sender.equals(from) || !text.equals(body) || kind == null) continue;
                if (room == null && kind == 1 && privateTabFor(text) >= 0) return true;
                if (room != null && kind == 3 && room.equals(target) && roomRows(room).contains(text)) return true;
            }
        }
        return false;
    }

    /** Require both an original-decoded404 and its completed roster dispatch. */
    private static boolean receivedPresence(String peer, byte presence) throws Exception {
        // bc.a(ay[]) owns this monitor while updating the roster. Acquiring it
        // avoids treating bytes observed by Connector as completed UI dispatch.
        synchronized (network) {
            Object roster = get(canvas, "Z");
            int count = (Integer) method(roster.getClass(), "c", int.class).invoke(roster);
            int contactID = -1;
            for (int index = 0; index < count; index++) {
                Object row = method(roster.getClass(), "e", Class.forName("t"), int.class).invoke(roster, index);
                if (row != null && peer.equals(field(row.getClass(), "e", String.class).get(row))) {
                    // Original au.a(I[B[BI)V sets key69; t maps it to j at240.
                    if (field(row.getClass(), "j", byte.class).getByte(row) != presence) return false;
                    contactID = field(row.getClass(), "a", int.class).getInt(row);
                    break;
                }
            }
            if (contactID == -1) return false;
            List<String> files = new ArrayList<>();
            synchronized (Connector.events) {
                for (String event : Connector.events) {
                    String[] columns = event.split("\t");
                    if (columns.length >= 5 && columns[0].equals("RX") && columns[1].equals("404")) files.add(columns[4]);
                }
            }
            Class<?> packetClass = Class.forName("ay");
            Class<?> arrayClass = java.lang.reflect.Array.newInstance(packetClass, 0).getClass();
            for (String filename : files) {
                Object[] packets = (Object[]) method(packetClass, "a", arrayClass, byte[].class)
                    .invoke(null, Files.readAllBytes(out.resolve(filename)));
                for (Object packet : packets) {
                    Integer id = (Integer) method(packetClass, "d", Integer.class, short.class).invoke(packet, (short) 1);
                    Byte value = (Byte) method(packetClass, "b", Byte.class, short.class).invoke(packet, (short) 2);
                    if (id != null && id == contactID && value != null && value == presence) return true;
                }
            }
            return false;
        }
    }

    public static void main(String[] args) {
        // Keep the protocol channel independent from the unchanged MIDlet's logs.
        System.setOut(System.err);
        try {
            if (args.length != 3 || (!args[1].equals("http") && !args[1].equals("tcp"))) {
                throw new IllegalArgumentException("Usage: V4MobilePeer <demo-user> <http|tcp> <output-directory>");
            }
            user = args[0];
            transport = args[1];
            out = Paths.get(args[2]).toAbsolutePath().normalize();
            Connector.out = out;
            init(false);
            emit("ready", "user", user, "transport", transport, "output", out.toString(),
                "proof", "original-login-roster-categories");

            boolean stopped = false;
            try (BufferedReader input = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8))) {
                String line;
                while ((line = input.readLine()) != null) {
                    String[] command = line.split("\t", -1);
                    if (command.length == 1 && command[0].equals("STOP")) {
                        logout();
                        emit("closed", "proof", "original-logout-ack");
                        stopped = true;
                        break;
                    }
                    if (command.length == 2 && command[0].equals("ROOM_JOIN")) {
                        String room = argument(command[1]);
                        join(room);
                        emit("room-joined", "room", room, "proof", "original-703-701-ack-and-ready-room-model");
                        continue;
                    }
                    if (command.length == 2 && command[0].equals("ROOM_LEAVE")) {
                        String room = argument(command[1]);
                        leave(room);
                        emit("room-left", "room", room, "proof", "original-704-ack-closed-tab-cleared-suppression");
                        continue;
                    }
                    if (command.length == 4 && command[0].equals("ROOM_EXPECT")) {
                        String room = argument(command[1]);
                        String sender = argument(command[2]);
                        String text = argument(command[3]);
                        until("original incoming room packet and UI model", () -> receivedMessage(sender, text, room));
                        selectRoom(room);
                        String capture = "mobile-room-received-" + (++roomCapture);
                        save(capture);
                        Files.writeString(out.resolve(capture + "-model.txt"), roomRows(room));
                        emit("room-received", "room", room, "from", sender, "text", text,
                            "proof", "original-ay-kind3-target-sender-and-room-model");
                        continue;
                    }
                    if (command.length == 3 && command[0].equals("ROOM_SEND")) {
                        String room = argument(command[1]);
                        String text = argument(command[2]);
                        int transaction = request(500, 1, () -> method(bc, "a", void.class, short.class,
                            String.class, String.class, int.class, byte[].class, short.class, int.class)
                            .invoke(network, (short) 1, room, text, 3, null, (short) 1, -1));
                        emit("room-sent", "room", room, "text", text, "transaction", Integer.toString(transaction),
                            "proof", "original-kind3-builder-and-correlated-ack");
                        continue;
                    }
                    if (command.length == 3 && command[0].equals("ROOM_PARTICIPANTS")) {
                        String room = argument(command[1]);
                        String names = argument(command[2]);
                        roomParticipants(room, "mobile-room-participants-" + (++roomCapture), names.split(";"));
                        emit("room-participants", "room", room, "names", names,
                            "proof", "original-707-708-and-participant-popup");
                        continue;
                    }
                    if (command.length != 3) throw new IllegalArgumentException("Expected SEND/EXPECT/PRESENCE with two base64 arguments, or STOP");
                    String peer = argument(command[1]);
                    String text = argument(command[2]);
                    if (command[0].equals("EXPECT")) {
                        until("original incoming chat and UI model", () -> receivedMessage(peer, text));
                        emit("received", "from", peer, "text", text, "proof", "original-ay-and-chat-model");
                    } else if (command[0].equals("SEND")) {
                        int transaction = request(500, 1, () -> send(peer, text));
                        emit("sent", "to", peer, "text", text, "transaction", Integer.toString(transaction),
                            "proof", "original-builder-and-correlated-ack");
                    } else if (command[0].equals("PRESENCE")) {
                        byte presence = Byte.parseByte(text);
                        until("original404 and roster presence " + peer + "=" + presence, () -> receivedPresence(peer, presence));
                        emit("presence", "peer", peer, "value", Byte.toString(presence), "proof", "original-404-and-roster-model");
                    } else {
                        throw new IllegalArgumentException("Unknown command " + command[0]);
                    }
                }
            }
            if (!stopped) {
                logout();
                emit("closed", "proof", "original-logout-ack");
            }
            System.exit(0);
        } catch (Throwable failure) {
            emit("error", "message", failure.toString());
            failure.printStackTrace(System.err);
            System.exit(1);
        }
    }
}
