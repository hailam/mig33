// Original 4.62 ay/bj classes only; no MIDlet startup or network operations.
// Compile: javac -cp research/applets/v4/artifacts/original/mig33-4.62.jar:tools/applets/bin/microemulator-2.0.4.jar -d /tmp/mig33-v4-protocol tools/applets/v4/protocol-fixture-harness.java
// Run: java -cp /tmp/mig33-v4-protocol:research/applets/v4/artifacts/original/mig33-4.62.jar:tools/applets/bin/microemulator-2.0.4.jar ProtocolV4FixtureHarness
class ProtocolV4FixtureHarness {
    static ay packet(int op,int tx){ay p=new ay((short)op);p.c=(short)tx;return p;}
    static void dump(String name,ay packet){StringBuilder b=new StringBuilder();for(byte v:packet.b())b.append(String.format("%02x",v&255));System.out.println(name+"="+b);}
    public static void main(String[] args){
        dump("ping",packet(2,1));
        ay error=packet(0,1);error.a(new bj((short)2,"Bad"));dump("error",error);
        dump("ack",packet(1,1));
        ay chat=packet(500,3);
        chat.a(new bj((short)1,(byte)1));chat.a(new bj((short)2,"alice"));chat.a(new bj((short)3,(byte)1));chat.a(new bj((short)4,"bob"));chat.a(new bj((short)5,42));chat.a(new bj((short)6,(short)1));chat.a(new bj((short)8,"Hello \u2713 \ud83d\ude00"));dump("chat_utf8",chat);
        ay dup=packet(101,5);dup.a(new bj((short)1,240));dup.a(new bj((short)1,320));dump("registration_duplicate_tag",dup);
        ay delta=packet(419,6);delta.a(new bj((short)1,1234567890123L));dump("contact_delta_long",delta);
        ay array=packet(407,7);array.a(new bj((short)1,42));array.a(new bj((short)30,new String[]{"alice@example.test","bob"}));array.a(new bj((short)31,new byte[]{2,4}));dump("contact_string_array",array);
        ay color=packet(500,8);color.a(new bj((short)1,(byte)1));color.a(new bj((short)2,"alice"));color.a(new bj((short)3,(byte)3));color.a(new bj((short)4,"fixture room"));color.a(new bj((short)6,(short)1));color.a(new bj((short)8,"hello"));color.a(new bj((short)13,0x112233));dump("colored_room_message",color);
        ay avatar=packet(602,9);avatar.a(new bj((short)1,"new-avatar"));avatar.a(new bj((short)1,"old-avatar"));dump("avatar_duplicate_tag",avatar);
    }
}
