package javax.microedition.io;
import java.io.*;
/** Offline capture sandbox: no original protocol can open a connection. */
public final class Connector {
 public static final int READ=1,WRITE=2,READ_WRITE=3;
 public static int blocked=0;
 public static Connection open(String n) throws IOException {blocked++;throw new ConnectionNotFoundException("Offline capture blocks all connections: "+n);}
 public static Connection open(String n,int m) throws IOException{return open(n);}
 public static Connection open(String n,int m,boolean t) throws IOException{return open(n);}
 public static InputStream openInputStream(String n) throws IOException{open(n);return null;}
 public static OutputStream openOutputStream(String n) throws IOException{open(n);return null;}
 public static DataInputStream openDataInputStream(String n) throws IOException{open(n);return null;}
 public static DataOutputStream openDataOutputStream(String n) throws IOException{open(n);return null;}
}
