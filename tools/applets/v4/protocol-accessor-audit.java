// Offline audit of the original 4.62 ay/bj accessors. No MIDlet or network startup.
// Compile and run through audit-transports.py; output is JSON Lines.
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;

class ProtocolAccessorAudit {
    static String quote(String value) {
        StringBuilder out = new StringBuilder("\"");
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            if (c == '"' || c == '\\') out.append('\\').append(c);
            else if (c < 32 || c > 126) out.append(String.format("\\u%04x", (int)c));
            else out.append(c);
        }
        return out.append('"').toString();
    }
    static String json(Object value) {
        if (value == null) return "null";
        if (value instanceof String) return quote((String)value);
        if (value instanceof String[]) {
            StringBuilder out = new StringBuilder("[");
            String[] values = (String[])value;
            for (int i = 0; i < values.length; i++) {
                if (i > 0) out.append(',');
                out.append(json(values[i]));
            }
            return out.append(']').toString();
        }
        return value.toString();
    }
    static Method method(String name, Class<?> result) throws Exception {
        for (Method method : bj.class.getMethods()) {
            if (method.getName().equals(name) && method.getReturnType() == result
                && method.getParameterCount() == 0) return method;
        }
        throw new NoSuchMethodException(name);
    }
    static String read(bj field, String name, Class<?> result) throws Exception {
        try { return json(method(name, result).invoke(field)); }
        catch (InvocationTargetException failure) {
            return "{\"throws\":" + quote(failure.getCause().getClass().getName()) + "}";
        }
    }
    static String hex(byte[] bytes) {
        StringBuilder out = new StringBuilder();
        for (byte value : bytes) out.append(String.format("%02x", value & 255));
        return out.toString();
    }
    static byte[] unhex(String text) {
        byte[] bytes = new byte[text.length()/2];
        for (int i = 0; i < bytes.length; i++) bytes[i]=(byte)Integer.parseInt(text.substring(i*2,i*2+2),16);
        return bytes;
    }
    static void dump(String name, bj field) throws Exception {
        System.out.println("{\"name\":"+quote(name)+",\"bytes\":"+quote(hex((byte[])method("i",byte[].class).invoke(field)))
            +",\"byte\":"+read(field,"d",Byte.class)+",\"short\":"+read(field,"g",Short.class)
            +",\"int\":"+read(field,"e",Integer.class)+",\"long\":"+read(field,"f",Long.class)
            +",\"text\":"+read(field,"h",String.class)+",\"text_array\":"+read(field,"j",String[].class)+"}");
    }
    static void frame(String name, String bytes) {
        try {
            ay[] packets = ay.a(unhex(bytes));
            String result = "null";
            if (packets != null) {
                StringBuilder out = new StringBuilder("[");
                for (int i = 0; i < packets.length; i++) {
                    if (i > 0) out.append(',');
                    out.append("{\"opcode\":").append(packets[i].a)
                        .append(",\"transaction\":").append(packets[i].c)
                        .append(",\"field_1_text\":").append(json(packets[i].f((short)1))).append('}');
                }
                result = out.append(']').toString();
            }
            System.out.println("{\"name\":"+quote(name)+",\"frame_bytes\":"+quote(bytes)+",\"packets\":"+result+"}");
        } catch (Exception failure) {
            System.out.println("{\"name\":"+quote(name)+",\"frame_bytes\":"+quote(bytes)
                +",\"throws\":"+quote(failure.getClass().getName())+"}");
        }
    }
    public static void main(String[] args) throws Exception {
        for (String bytes : new String[]{"", "00", "ff", "0001", "ffff", "00010002", "ffffffff",
                "0000000000000001", "ffffffffffffffff", "000000010203040506"}) {
            dump("raw_"+bytes, new bj((short)31,unhex(bytes)));
        }
        for (String value : new String[]{"a", "a\0b", "a\0bc", "ab\0c", "\0a", "a\0", "a\0\0b",
                "ab\0cd\0ef", "alice@example.test\0bob"}) {
            dump("array_"+hex(value.getBytes(StandardCharsets.UTF_8)),new bj((short)30,value.getBytes(StandardCharsets.UTF_8)));
        }
        dump("typed_int_65538",new bj((short)31,65538));
        ay packet=new ay((short)203);
        packet.a(new bj((short)31,65538));
        ay decoded=ay.a(packet.b())[0];
        dump("wire_int_65538",decoded.a((short)31));
        System.out.println("{\"name\":\"missing_field\",\"byte\":"+json(decoded.b((short)99))
            +",\"short\":"+json(decoded.c((short)99))+",\"int\":"+json(decoded.d((short)99))
            +",\"long\":"+json(decoded.e((short)99))+",\"text\":"+json(decoded.f((short)99))
            +",\"text_array\":"+json(decoded.h((short)99))+"}");
        // Small bounded inputs only: no oversized allocation probes.
        frame("empty_stream", "");
        frame("empty_frame", "020002000100000000");
        frame("two_frames", "020002000100000000020003000200000000");
        frame("invalid_marker", "030002000100000000");
        frame("truncated_header", "020002");
        frame("negative_body_length", "0200020001ffffffff");
        frame("negative_field_length", "0200020001000000060001ffffffff");
        frame("body_shorter_than_field", "02000200010000000100010000000161");
        frame("truncated_field", "020002000100000007000100000001");
        frame("duplicate_field_last_wins", "02000200010000000e0001000000016100010000000162");
        frame("signed_envelope_values", "02ffffffff00000000");
    }
}
