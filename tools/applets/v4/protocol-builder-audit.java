// Offline audit of original bc request builders. Does not construct/start bc,
// initialize a MIDlet, or call a transport. The original private queue is captured.
// Compile/run with the verified original applet and MicroEmulator on the classpath.
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.Vector;
import sun.misc.Unsafe;

class ProtocolBuilderAudit {
    static Object builder;
    static Vector queue;
    static Class<?> type;
    static void set(Class<?> owner, Object instance, String name, Object value) throws Exception {
        Field field = owner.getDeclaredField(name);
        field.setAccessible(true);
        field.set(instance, value);
    }
    static void call(String label, String name, Class<?>[] parameters, Object... args) throws Exception {
        queue.clear();
        Method method = type.getDeclaredMethod(name, parameters);
        method.setAccessible(true);
        method.invoke(builder, args);
        if (queue.size() != 1) throw new AssertionError(label + " queue size " + queue.size());
        Object packet = queue.elementAt(0);
        byte[] bytes = (byte[]) packet.getClass().getMethod("b").invoke(packet);
        StringBuilder hex = new StringBuilder();
        for (byte value : bytes) hex.append(String.format("%02x", value & 255));
        System.out.println(label + "=" + hex);
    }
    public static void main(String[] args) throws Exception {
        Field unsafeField = Unsafe.class.getDeclaredField("theUnsafe");
        unsafeField.setAccessible(true);
        Unsafe unsafe = (Unsafe) unsafeField.get(null);
        type = Class.forName("bc");
        builder = unsafe.allocateInstance(type);
        queue = new Vector();
        Field queueField = type.getDeclaredField("r");
        queueField.setAccessible(true);
        queueField.set(builder, queue);
        Class<?> S=String.class, I=int.class, B=byte.class, H=short.class, Z=boolean.class;
        call("help", "a", new Class<?>[]{I}, 5);
        call("security", "d", new Class<?>[]{S}, "answer");
        call("avatar", "d", new Class<?>[]{S,S}, "new", "old");
        call("status", "c", new Class<?>[]{S,S}, "new", "old");
        call("block_true", "b", new Class<?>[]{S,Z}, "peer", true);
        call("block_false", "b", new Class<?>[]{S,Z}, "peer", false);
        call("join_true", "a", new Class<?>[]{S,Z}, "room", true);
        call("join_false", "a", new Class<?>[]{S,Z}, "room", false);
        call("callback", "a", new Class<?>[]{S,S,I,I}, "from", "to", 42, 99);
        call("callback_changed", "a", new Class<?>[]{S,S,I,I}, "from", "to", -7, 0);
        call("connect_true", "a", new Class<?>[]{H,I,I,Z}, (short)206, 3, 1, true);
        call("connect_false", "a", new Class<?>[]{H,I,I,Z}, (short)206, 3, 1, false);
        call("disconnect", "a", new Class<?>[]{H,I,I,Z}, (short)302, 3, 99, true);
        call("prompt", "a", new Class<?>[]{B,H}, (byte)1, (short)99);
        call("presence", "a", new Class<?>[]{I,I}, 4, 1);
        call("emoticon", "a", new Class<?>[]{S,I}, ":)", 1);
        call("category", "a", new Class<?>[]{H,B}, (short)7, (byte)1);
        call("remove_favorite", "a", new Class<?>[]{S,H}, "room", (short)7);
        call("rename_group", "b", new Class<?>[]{S,I}, "group", 42);
        call("move_group", "b", new Class<?>[]{I,I}, 42, 7);
        call("move_no_group", "b", new Class<?>[]{I,I}, 42, -1);
        call("activity_on", "a", new Class<?>[]{B}, (byte)1);
        call("activity_off", "a", new Class<?>[]{B}, (byte)0);
        call("private_group_new", "a", new Class<?>[]{S,S,S,B}, "peer", "invitee", null, (byte)3);
        call("private_group_invite", "a", new Class<?>[]{S,S,S,B}, "peer", "invitee", "group", (byte)3);
        call("private_group_leave", "a", new Class<?>[]{S,B}, "group", (byte)3);
        call("private_group_members", "b", new Class<?>[]{S,B}, "group", (byte)3);
        call("invite_friend", "b", new Class<?>[]{S,S}, "mobile", "name");
        call("forgot", "h", new Class<?>[]{S}, "mobile");
        call("add_contact_group", "j", new Class<?>[]{S}, "group");
        call("add_favorite", "i", new Class<?>[]{S}, "room");
        call("language", "a", new Class<?>[]{S}, "en-US");
        call("language_list_job", "b", new Class<?>[]{S}, "http://example.test/list");
        call("language_pack_job", "c", new Class<?>[]{S}, "http://example.test/pack");
        call("raw_http_job", "g", new Class<?>[]{S}, "http://example.test/raw");
        call("room_categories", "b", new Class<?>[]{});
        call("generic_empty", "a", new Class<?>[]{H}, (short)400);
        call("generic_text", "a", new Class<?>[]{H,S}, (short)704, "room");
        call("generic_int", "a", new Class<?>[]{H,I}, (short)406, 42);
        call("generic_member", "a", new Class<?>[]{H,S,S}, (short)706, "target", "room");
        call("generic_test", "a", new Class<?>[]{H}, (short)2);
        call("generic_logout", "a", new Class<?>[]{H}, (short)300);
        call("generic_activate", "a", new Class<?>[]{H,S}, (short)204, "code");
        call("generic_mobile", "a", new Class<?>[]{H,S}, (short)205, "mobile");
        call("generic_delete_group", "a", new Class<?>[]{H,I}, (short)409, 7);
        call("generic_participants", "a", new Class<?>[]{H,S}, (short)707, "room");
        call("generic_mute", "a", new Class<?>[]{H,S,S}, (short)709, "target", "room");
        call("generic_unmute", "a", new Class<?>[]{H,S,S}, (short)710, "target", "room");
        call("preference_present", "a", new Class<?>[]{B,S,S}, (byte)2, "key", "value");
        call("preference_absent", "a", new Class<?>[]{B,S,S}, (byte)2, "key", null);
        call("upload_parameter", "a", new Class<?>[]{H,byte[].class,H,B}, (short)8, new byte[]{0,1,-1}, (short)7, (byte)0);
        call("upload_no_parameter", "a", new Class<?>[]{H,byte[].class,H,B}, (short)1, new byte[]{0,1,-1}, (short)-1, (byte)1);
        Vector names = new Vector(); names.addElement("alice"); names.addElement("bob");
        Vector services = new Vector(); services.addElement("3"); services.addElement("4");
        Class<?>[] edit = {S,S,S,Vector.class,Vector.class,I,I};
        call("edit_contact_full", "a", edit, "display", "mobile", "mig33", names, services, 42, 7);
        call("edit_contact_min", "a", edit, null, null, null, null, null, 42, -1);
        services.removeElementAt(0);
        call("edit_contact_mismatch", "a", edit, null, null, null, names, services, 42, -1);
        Vector empty = new Vector();
        call("edit_contact_empty", "a", edit, null, null, null, empty, empty, 42, -1);
        Class<?>[] add = {S,S,S,S,S,S,S,S,I,Z,S,S,B};
        call("add_contact_full", "a", add, "display", "mobile", "mig33", "unused4", "unused5", "unused6", "unused7", "unused8", 7, true, "unused11", "external", (byte)3);
        call("add_contact_min", "a", add, null, null, null, null, null, null, null, null, -1, false, null, null, (byte)-1);
        Class<?> ui = Class.forName("aj");
        set(ui, null, "e", 240);
        set(ui, null, "f", 320);
        call("registration_start", "a", new Class<?>[]{});
        set(ui, null, "c", "4.62");
        set(ui, null, "a", "-test");
        call("registration_submit", "a", new Class<?>[]{S,S,S,S,S}, "user", "password", "mobile", "code", "email");
        Object uiInstance = unsafe.allocateInstance(ui);
        set(type, builder, "u", uiInstance);
        Class<?> packetType = Class.forName("ay");
        Object cached = packetType.getConstructor(H).newInstance((short)906);
        Class<?> fieldType = Class.forName("bj");
        Method insert = packetType.getMethod("a", fieldType);
        insert.invoke(cached, fieldType.getConstructor(H,S).newInstance((short)2, "retained"));
        insert.invoke(cached, fieldType.getConstructor(H,B).newInstance((short)30, (byte)3));
        set(ui, uiInstance, "aD", cached);
        call("cached_settings", "b", new Class<?>[]{B}, (byte)9);
        call("cached_settings_changed_argument", "b", new Class<?>[]{B}, (byte)0);
    }
}
