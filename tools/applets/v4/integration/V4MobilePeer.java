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
 * arguments: EXPECT peer text, SEND peer text, or STOP (without arguments).
 */
public final class V4MobilePeer extends V4GroupIntegration {
    private static final PrintStream replies = System.out;

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
                if (sender.equals(from) && text.equals(body) && privateTabFor(text) >= 0) return true;
            }
        }
        return false;
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
                    if (command.length != 3) throw new IllegalArgumentException("Expected SEND/EXPECT with two base64 arguments, or STOP");
                    String peer = argument(command[1]);
                    String text = argument(command[2]);
                    if (command[0].equals("EXPECT")) {
                        until("original incoming chat and UI model", () -> receivedMessage(peer, text));
                        emit("received", "from", peer, "text", text, "proof", "original-ay-and-chat-model");
                    } else if (command[0].equals("SEND")) {
                        int transaction = request(500, 1, () -> send(peer, text));
                        emit("sent", "to", peer, "text", text, "transaction", Integer.toString(transaction),
                            "proof", "original-builder-and-correlated-ack");
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
