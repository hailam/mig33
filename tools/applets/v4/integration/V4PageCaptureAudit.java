import java.awt.image.BufferedImage;
import java.nio.file.*;
import java.util.*;
import javax.imageio.ImageIO;

/** Independently checks every saved Canvas capture after the native JVMs exit. */
public class V4PageCaptureAudit {
    public static void main(String[] args) throws Exception {
        Path root = Paths.get(args[0]);
        List<String> records = new ArrayList<>();
        records.add("capture\tcolors\tdominant_fraction\tpopulated\tintentional_empty_chat");
        try (var paths = Files.walk(root)) {
            for (Path path : paths.filter(p -> p.toString().endsWith(".png")).sorted().toList()) {
                BufferedImage image = ImageIO.read(path.toFile());
                if (image.getWidth() != 240 || image.getHeight() != 320)
                    throw new AssertionError("Unexpected Canvas dimensions: " + path);
                Map<Integer, Integer> counts = new HashMap<>();
                for (int y = 24; y < 306; y++) for (int x = 4; x < 232; x++)
                    counts.merge(image.getRGB(x, y) & 0xffffff, 1, Integer::sum);
                double dominant = (double) Collections.max(counts.values()) / (228 * 282);
                // Two-color controls (the orange/white palette grid) are valid;
                // a uniform gray or white body still fails the density check.
                boolean populated = dominant < 0.995 && counts.size() >= 2;
                // The theme test intentionally captures a new empty Bob tab
                // to make the changed background color directly observable.
                boolean intentional = path.getFileName().toString().equals("native-theme-chat-background.png");
                records.add(root.relativize(path) + "\t" + counts.size() + "\t" +
                    String.format(Locale.ROOT, "%.8f", dominant) + "\t" + populated + "\t" + intentional);
                if (!populated && !intentional)
                    throw new AssertionError("Blank native content-area capture: " + path);
            }
        }
        Files.write(root.resolve("capture-pixel-audit.tsv"), records);
        System.out.println("PASS pixel audit for all " + (records.size() - 1) + " Canvas captures");
    }
}
