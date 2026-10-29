# MicroEmulator RMS compatibility

`org/microemu/util/RecordStoreImpl.java` is the MicroEmulator2.0.4 source, retaining its Apache2/LGPL2.1 dual-license header. Source archive: [Maven Central publisher artifact](https://repo.maven.apache.org/maven2/org/microemu/microemulator/2.0.4/microemulator-2.0.4-sources.jar). Only open/close reference counting and visibility/synchronization are changed.

The original mig33 `aj.d(ay)` opens store52 while `aj.n(int)` opens and closes the same store during icon lookup. MicroEmulator's original boolean closes the outer handle prematurely, producing `RecordStoreNotOpenException` before saving the received927 icon. MIDP requires balanced closes for nested opens: [Oracle RecordStore contract](https://docs.oracle.com/javame/config/cldc/ref-impl/midp2.0/jsr118/javax/microedition/rms/RecordStore.html).

The replacement emulator class retains the same object across opens, decrements a reference count on close, and only closes/removes listeners at the final balanced close. It remains an in-memory emulator store under these runners; server persistence is unaffected. The archived mig33 JAR is unchanged.

Run `sh tools/applets/v4/integration/run.sh --compile-only`, then compile/run `RMSReferenceCountTest.java` with `data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar` as the classpath. Native media acceptance exercises the original nested icon-cache path as well.
