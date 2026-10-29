package javax.microedition.io;
import java.io.*;
import java.net.*;
import java.nio.file.*;
import java.util.*;
/** Test bridge: permits only the two explicit loopback backend listeners. */
public final class Connector {
 public static final int HTTP_PORT=port("MIG33_TEST_HTTP_PORT",8080),TCP_PORT=port("MIG33_TEST_TCP_PORT",9119);
 static int port(String key,int fallback){String value=System.getenv(key);int result=value==null?fallback:Integer.parseInt(value);if(result<1024||result>65535)throw new IllegalArgumentException("Invalid loopback test port");return result;}
 public static final int READ=1,WRITE=2,READ_WRITE=3;
 public static int blocked=0;
 public static final List<String> events=Collections.synchronizedList(new ArrayList<String>());
 public static Path out=Paths.get("data/applets/integration/out");
 static final Frames httpTX=new Frames("TX"),httpRX=new Frames("RX");
 public static volatile int emptyPolls=0;
 static synchronized void log(String s){events.add(s);System.out.println(s);try{Files.writeString(out.resolve("events.tsv"),s+"\n",StandardOpenOption.CREATE,StandardOpenOption.APPEND);}catch(IOException e){throw new RuntimeException(e);}}
 public static int count(String direction,int opcode){int n=0;synchronized(events){for(String e:events)if(e.startsWith(direction+"\t"+opcode+"\t"))n++;}return n;}
 public static Connection open(String n)throws IOException{return open(n,3,false);}
 public static Connection open(String n,int m)throws IOException{return open(n,m,false);}
 public static Connection open(String n,int m,boolean t)throws IOException{
  URI u;try{u=URI.create(n);}catch(IllegalArgumentException e){throw new ConnectionNotFoundException("Invalid connection URI");}
  // This public lookup is hard-coded in the unmodified JAR, independent of
  // its configured gateway. Rewrite only that exact historical endpoint.
  if("http".equals(u.getScheme())&&"www.mig33.com".equals(u.getHost())&&u.getUserInfo()==null&&u.getPort()==-1&&"/sites/index.php".equals(u.getPath())&&u.getRawQuery()!=null&&u.getRawQuery().startsWith("c=registration_api&v=midlet&a=check_username&username=")){
   n="http://127.0.0.1:"+HTTP_PORT+u.getRawPath()+"?"+u.getRawQuery();u=URI.create(n);log("REWRITE_REGISTRATION\t"+n);
  }
  if("http".equals(u.getScheme())&&"www.mig33.com".equals(u.getHost())&&u.getUserInfo()==null&&u.getPort()==-1&&"/sites/resources/language/v4.6/l".equals(u.getPath())&&u.getRawQuery()==null){
   n="http://127.0.0.1:"+HTTP_PORT+u.getRawPath();u=URI.create(n);log("REWRITE_LANGUAGE\t"+n);
  }
  // Existing harnesses configure the original loopback defaults. Explicit test
  // ports remap only these two endpoints, keeping the main backend untouched.
  if("127.0.0.1".equals(u.getHost())&&u.getUserInfo()==null&&("http".equals(u.getScheme())&&u.getPort()==8080||"socket".equals(u.getScheme())&&u.getPort()==9119)){
   n=u.getScheme()+"://127.0.0.1:"+("http".equals(u.getScheme())?HTTP_PORT:TCP_PORT)+(u.getRawPath()==null?"":u.getRawPath())+(u.getRawQuery()==null?"":"?"+u.getRawQuery());u=URI.create(n);
  }
  if(!"127.0.0.1".equals(u.getHost()) || u.getUserInfo()!=null || !("socket".equals(u.getScheme())&&u.getPort()==TCP_PORT || "http".equals(u.getScheme())&&u.getPort()==HTTP_PORT)){
   blocked++;log("BLOCK\t"+n);throw new ConnectionNotFoundException("Integration bridge blocks non-test endpoint: "+n);
  }
  log("OPEN\t"+n);
  if("socket".equals(u.getScheme()))return new LocalSocket();
  return new LocalHttp(u.toURL());
 }
 public static InputStream openInputStream(String n)throws IOException{return ((InputConnection)open(n)).openInputStream();}
 public static OutputStream openOutputStream(String n)throws IOException{return ((OutputConnection)open(n)).openOutputStream();}
 public static DataInputStream openDataInputStream(String n)throws IOException{return new DataInputStream(openInputStream(n));}
 public static DataOutputStream openDataOutputStream(String n)throws IOException{return new DataOutputStream(openOutputStream(n));}
 static final class Frames {
  static final java.util.concurrent.atomic.AtomicInteger sequence=new java.util.concurrent.atomic.AtomicInteger();
  final String direction;byte[] pending=new byte[0];
  Frames(String d){direction=d;}
  synchronized void add(byte[] b,int off,int n)throws IOException{
   byte[] next=Arrays.copyOf(pending,pending.length+n);System.arraycopy(b,off,next,pending.length,n);pending=next;
   while(pending.length>=9){
    int len=((pending[5]&255)<<24)|((pending[6]&255)<<16)|((pending[7]&255)<<8)|(pending[8]&255);
    if(pending[0]!=2||len<0||len>10000000)throw new IOException("Invalid binary frame in "+direction);
    if(pending.length<9+len)return;
    int opcode=((pending[1]&255)<<8)|(pending[2]&255),trx=((pending[3]&255)<<8)|(pending[4]&255);
    byte[] packet=Arrays.copyOf(pending,9+len);pending=Arrays.copyOfRange(pending,9+len,pending.length);
    String fn=direction+"-"+String.format("%05d",sequence.incrementAndGet())+"-"+opcode+".bin";
    Files.write(out.resolve(fn),packet);log(direction+"\t"+opcode+"\t"+trx+"\t"+len+"\t"+fn);
   }
  }
 }
 static InputStream observe(InputStream in,Frames f){return new FilterInputStream(in){public int read()throws IOException{int r=in.read();if(r>=0)f.add(new byte[]{(byte)r},0,1);return r;}public int read(byte[] b,int o,int n)throws IOException{int r=in.read(b,o,n);if(r>0)f.add(b,o,r);return r;}};}
 static OutputStream observe(OutputStream out,Frames f){return new FilterOutputStream(out){public void write(int b)throws IOException{out.write(b);f.add(new byte[]{(byte)b},0,1);}public void write(byte[] b,int o,int n)throws IOException{out.write(b,o,n);f.add(b,o,n);}};}
 static final class LocalSocket implements StreamConnection {
  final Socket s=new Socket();final Frames tx=new Frames("TX"),rx=new Frames("RX");
  LocalSocket()throws IOException{s.connect(new InetSocketAddress("127.0.0.1",TCP_PORT),5000);s.setTcpNoDelay(true);}
  public InputStream openInputStream()throws IOException{return observe(s.getInputStream(),rx);}
  public OutputStream openOutputStream()throws IOException{return observe(s.getOutputStream(),tx);}
  public DataInputStream openDataInputStream()throws IOException{return new DataInputStream(openInputStream());}
  public DataOutputStream openDataOutputStream()throws IOException{return new DataOutputStream(openOutputStream());}
  public void close()throws IOException{s.close();}
 }
 static final class LocalHttp implements HttpConnection {
  final HttpURLConnection c;final URL url;final boolean gateway;
  LocalHttp(URL u)throws IOException{url=u;gateway=!u.getPath().startsWith("/sites/")&&!u.getPath().startsWith("/assets/")&&!u.getPath().startsWith("/api/");c=(HttpURLConnection)u.openConnection();c.setInstanceFollowRedirects(false);c.setConnectTimeout(5000);c.setReadTimeout(15000);}
  public String getURL(){return url.toString();}public String getProtocol(){return url.getProtocol();}public String getHost(){return url.getHost();}public String getFile(){return url.getFile();}public String getRef(){return url.getRef();}public String getQuery(){return url.getQuery();}public int getPort(){return url.getPort();}
  public String getRequestMethod(){return c.getRequestMethod();}public void setRequestMethod(String m)throws IOException{c.setRequestMethod(m);}public String getRequestProperty(String k){return c.getRequestProperty(k);}public void setRequestProperty(String k,String v)throws IOException{c.setRequestProperty(k,v);}
  public int getResponseCode()throws IOException{return c.getResponseCode();}public String getResponseMessage()throws IOException{return c.getResponseMessage();}public long getExpiration(){return c.getExpiration();}public long getDate(){return c.getDate();}public long getLastModified(){return c.getLastModified();}
  public String getHeaderField(String k){return c.getHeaderField(k);}public int getHeaderFieldInt(String k,int d){return c.getHeaderFieldInt(k,d);}public long getHeaderFieldDate(String k,long d){return c.getHeaderFieldDate(k,d);}public String getHeaderField(int k){return c.getHeaderField(k);}public String getHeaderFieldKey(int k){return c.getHeaderFieldKey(k);}
  public String getType(){return c.getContentType();}public String getEncoding(){return c.getContentEncoding();}public long getLength(){return c.getContentLengthLong();}
  public InputStream openInputStream()throws IOException{
   InputStream input=c.getInputStream();log("HTTP\t"+c.getRequestMethod()+"\t"+c.getResponseCode()+"\t"+c.getContentType()+"\t"+c.getContentLengthLong());
   InputStream observed=gateway?observe(input,httpRX):input;
   if(!gateway||!"GET".equals(c.getRequestMethod()))return observed;
   return new FilterInputStream(observed){int bytes;boolean ended;void record(int n){if(n>0)bytes+=n;else if(n<0&&!ended){ended=true;if(bytes==0){emptyPolls++;log("HTTP_EMPTY_POLL\t200\t"+c.getContentType());}}}public int read()throws IOException{int r=in.read();record(r<0?-1:1);return r;}public int read(byte[] b,int o,int n)throws IOException{int r=in.read(b,o,n);record(r);return r;}};
  }public DataInputStream openDataInputStream()throws IOException{return new DataInputStream(openInputStream());}
  public OutputStream openOutputStream()throws IOException{if("GET".equals(c.getRequestMethod()))return new ByteArrayOutputStream(){public void write(int b){throw new IllegalStateException("GET body not supported");}public void write(byte[] b,int o,int n){if(n!=0)throw new IllegalStateException("GET body not supported");}};c.setDoOutput(true);OutputStream o=c.getOutputStream();return gateway?observe(o,httpTX):o;}public DataOutputStream openDataOutputStream()throws IOException{return new DataOutputStream(openOutputStream());}public void close(){c.disconnect();}
 }
}
