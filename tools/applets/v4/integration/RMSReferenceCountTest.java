import javax.microedition.rms.*;
import org.microemu.util.MemoryRecordStoreManager;

/** Checks the emulator compatibility fix against the nested open used by aj.d/aj.n. */
public final class RMSReferenceCountTest {
    public static void main(String[] args) throws Exception {
        MemoryRecordStoreManager manager = new MemoryRecordStoreManager();
        RecordStore outer = manager.openRecordStore("52", true);
        RecordStore inner = manager.openRecordStore("52", true);
        if (outer != inner) throw new AssertionError("Nested open must return the same object");
        inner.closeRecordStore();
        int id = outer.addRecord(new byte[]{1, 2, 3}, 0, 3);
        if (outer.getRecord(id)[2] != 3) throw new AssertionError("Inner close invalidated outer handle");
        outer.closeRecordStore();
        try { outer.getNumRecords(); throw new AssertionError("Final close left store open"); }
        catch (RecordStoreNotOpenException expected) {}
        try { outer.closeRecordStore(); throw new AssertionError("Unbalanced close accepted"); }
        catch (RecordStoreNotOpenException expected) {}
        RecordStore reopened = manager.openRecordStore("52", false);
        if (reopened.getNumRecords() != 1) throw new AssertionError("Reopen lost records");
        reopened.closeRecordStore();
        System.out.println("PASS nested RMS opens, final closure, unbalanced close, and reopen persistence");
    }
}
