import java.lang.reflect.*;
public class FieldProbe {
 static Method m(Class<?> c,String n,Class<?> r)throws Exception{for(Method x:c.getMethods())if(x.getName().equals(n)&&x.getReturnType()==r&&x.getParameterCount()==0)return x;throw new Exception(n);}
 public static void main(String[] args)throws Exception{
 Class<?> c=Class.forName("bj");
 for(byte[] raw:new byte[][]{{0,0},{0,0,0,0},{}}){Object f=c.getConstructor(short.class,byte[].class).newInstance((short)31,raw);System.out.println("rawLength="+raw.length+" int="+m(c,"e",Integer.class).invoke(f)+" short="+m(c,"g",Short.class).invoke(f)+" string="+m(c,"h",String.class).invoke(f));}
 }
}
