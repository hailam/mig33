// Compile: javac -cp research/applets/v3/artifacts/original/mig33-3.08.jar -d /tmp/mig33-protocol tools/applets/v3/protocol-fixture-harness.java
// Run: java -cp /tmp/mig33-protocol:research/applets/v3/artifacts/original/mig33-3.08.jar ProtocolFixtureHarness
// Uses ONLY original standalone field/packet classes p and w. No MIDlet/network APIs.
class ProtocolFixtureHarness {
    static void dump(String name, w packet) {
        StringBuilder result = new StringBuilder();
        for (byte b : packet.e()) result.append(String.format("%02x", b & 255));
        System.out.println(name + "=" + result);
    }
    static w packet(int op, int tx) {
        w result = new w((short) op);
        result.a((short) tx);
        return result;
    }
    public static void main(String[] args) {
        dump("ping", packet(2, 1));
        w error = packet(0, 1);
        error.a(new p((short)2, "Bad"));
        dump("error", error);
        dump("ack", packet(1, 1));
        w login = packet(200, 2);
        login.a(new p((short)1, (short)1));
        login.a(new p((short)2, (byte)2));
        login.a(new p((short)3, (short)308));
        login.a(new p((short)5, "fixture_user"));
        login.a(new p((short)7, "J2MEv3.08.165"));
        login.a(new p((short)8, "FixturePhone"));
        login.a(new p((short)9, (byte)99));
        login.a(new p((short)11, 12));
        login.a(new p((short)12, 234));
        login.a(new p((short)13, 304));
        login.a(new p((short)14, -1));
        dump("login", login);
        w chat = packet(500, 3);
        chat.a(new p((short)1, (byte)1));
        chat.a(new p((short)2, "alice"));
        chat.a(new p((short)3, (byte)1));
        chat.a(new p((short)4, "bob"));
        chat.a(new p((short)5, 42));
        chat.a(new p((short)6, (short)1));
        chat.a(new p((short)8, "Hello \u2713 \ud83d\ude00"));
        dump("chat_utf8", chat);
        w image = packet(504, 4);
        image.a(new p((short)1, (short)1));
        image.a(new p((short)2, new byte[]{0,1,2,127,(byte)128,(byte)255}));
        image.a(new p((short)4, (short)1));
        dump("image_bytes", image);
        w duplicate = packet(101, 5);
        duplicate.a(new p((short)1, 240));
        duplicate.a(new p((short)1, 320));
        dump("registration_duplicate_tag", duplicate);
        String[] strings = {"", "abc", "nonce-42password", "\ud83d\ude00", "\u0000\u00e9"};
        for (int i = 0; i < strings.length; i++) System.out.println("hash" + i + "=" + strings[i].hashCode());
    }
}
