import java.awt.image.BufferedImage;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import javax.imageio.ImageIO;
import javax.microedition.io.Connector;
import javax.microedition.lcdui.Display;
import javax.swing.SwingUtilities;
import org.microemu.MIDletBridge;
import org.microemu.app.Main;
import org.microemu.device.j2se.J2SEDevice;

/** Interactive original MIDlet, with the existing loopback-only transport adapter. */
public final class V4Desktop extends Main {
    private static String title(String label, String state) {
        return "mig33 v4.62 — " + (label.isEmpty() ? "" : label + " — ") + state;
    }

    private void positionWindow(int index) {
        if (index == 0) {
            setLocationByPlatform(true);
            return;
        }
        java.awt.GraphicsConfiguration config = getGraphicsConfiguration();
        java.awt.Rectangle screen = config.getBounds();
        java.awt.Insets insets = java.awt.Toolkit.getDefaultToolkit().getScreenInsets(config);
        int left = screen.x + insets.left;
        int top = screen.y + insets.top;
        int width = screen.width - insets.left - insets.right;
        int height = screen.height - insets.top - insets.bottom;
        int gap = 24;
        int start = left + Math.max(0, (width - 2 * getWidth() - gap) / 2);
        int x = Math.min(start + (index - 1) * (getWidth() + gap), left + Math.max(0, width - getWidth()));
        setLocation(x, top + Math.max(0, (height - getHeight()) / 2));
    }

    private static Field field(Class<?> type, String name, Class<?> valueType) throws Exception {
        for (Field f : type.getDeclaredFields()) {
            if (f.getName().equals(name) && f.getType() == valueType) {
                f.setAccessible(true);
                return f;
            }
        }
        throw new NoSuchFieldException(name);
    }

    private static Method method(Class<?> type, String name, Class<?> result, Class<?>... args) throws Exception {
        for (Method m : type.getDeclaredMethods()) {
            if (m.getName().equals(name) && m.getReturnType() == result && Arrays.equals(m.getParameterTypes(), args)) {
                m.setAccessible(true);
                return m;
            }
        }
        throw new NoSuchMethodException(name);
    }

    public static void main(String[] args) {
        try {
            String user = args.length == 0 ? "" : args[0];
            String label = System.getProperty("mig33.client.label", "");
            int index = Integer.parseInt(System.getProperty("mig33.client.index", "0"));
            if (index < 0 || index > 2) throw new IllegalArgumentException("Client index must be 0, 1 or 2.");
            if (!user.isEmpty() && !Arrays.asList("alice", "bob", "carol", "dave", "hailam").contains(user)) {
                throw new IllegalArgumentException("Use alice, bob, carol, dave, hailam, or no argument for the login screen.");
            }
            Path output = Path.of(System.getProperty("mig33.client.output"));
            Files.createDirectories(output);
            Connector.out = output;
            try {
                Connector.open("http://interactive-proof.invalid");
                throw new AssertionError("Wrong Connector: external network was allowed");
            } catch (javax.microedition.io.ConnectionNotFoundException expected) {
                // The same adapter used by the integration tests refuses legacy hosts.
            }
            V4Desktop[] holder = new V4Desktop[1];
            SwingUtilities.invokeAndWait(() -> {
                V4Desktop window = new V4Desktop();
                holder[0] = window;
                window.common.initParams(new ArrayList<String>(Arrays.asList(
                    "--rms", "memory", "--appclassloader", "system", "main.Main")),
                    window.selectDevicePanel.getSelectedDeviceEntry(), J2SEDevice.class);
                window.updateDevice();
                window.setTitle(title(label, "local demo"));
                window.positionWindow(index);
                window.setVisible(true);
            });
            V4Desktop window = holder[0];
            window.common.initMIDlet(true);
            Thread.sleep(1600);
            Object canvas = Display.getDisplay(MIDletBridge.getCurrentMIDlet()).getCurrent();
            Class<?> controller = Class.forName("aj"), transport = Class.forName("bc");
            field(controller, "c", String.class).set(null, "4.62");
            field(controller, "a", String.class).set(null, ".000");
            field(controller, "b", int.class).setInt(null, 2);
            field(transport, "e", String.class).set(null, "127.0.0.1");
            field(transport, "f", String.class).set(null, "9119");
            field(transport, "d", String.class).set(null, "http://127.0.0.1:8080/");
            method(controller, "c", void.class).invoke(canvas);
            if (!user.isEmpty()) {
                String password = user.equals("hailam") ? "hailam" : user + "-demo";
                method(controller, "f", void.class, String.class, String.class).invoke(canvas, user, password);
                long deadline = System.currentTimeMillis() + 20000;
                Field progress = field(controller, "aG", Class.forName("y"));
                while ((Connector.count("RX", 403) == 0 || progress.get(canvas) != null) && System.currentTimeMillis() < deadline) Thread.sleep(100);
                if (Connector.count("RX", 203) == 0 || Connector.count("RX", 403) == 0 || progress.get(canvas) != null) {
                    throw new IllegalStateException("Local login did not finish; check that mig33d is running.");
                }
                Class<?> tabsType = Class.forName("k");
                Object tabs = field(controller, "Y", tabsType).get(canvas);
                int friends = (Integer) method(tabsType, "b", int.class, int.class).invoke(tabs, 1);
                if (friends >= 0) method(tabsType, "g", void.class, int.class).invoke(tabs, friends);
                System.out.println("READY: original v4.62 logged in as " + user);
            } else {
                System.out.println("READY: original login screen; hailam / hailam, alice / alice-demo (moderator), bob / bob-demo");
            }
            method(controller, "a", void.class, int.class).invoke(canvas, Integer.MAX_VALUE);
            Thread.sleep(600);
            SwingUtilities.invokeAndWait(() -> {
                window.setTitle(title(label, user.isEmpty() ? "local login" : user + " (local)"));
                window.toFront();
                window.requestFocus();
                try {
                    BufferedImage preview = new BufferedImage(window.getWidth(), window.getHeight(), BufferedImage.TYPE_INT_RGB);
                    java.awt.Graphics2D graphics = preview.createGraphics();
                    window.paint(graphics);
                    graphics.dispose();
                    ImageIO.write(preview, "png", output.resolve("window.png").toFile());
                } catch (Exception error) {
                    error.printStackTrace();
                }
            });
            Files.writeString(output.resolve("ready.txt"), user.isEmpty() ? "Local login screen; signed out.\n" : "Local login completed as " + user + ".\n");
            System.out.println("Controls: arrows, Enter, F1 left softkey, F2 right softkey; or click the phone keypad.");
        } catch (Throwable error) {
            error.printStackTrace();
            System.exit(1);
        }
    }
}
