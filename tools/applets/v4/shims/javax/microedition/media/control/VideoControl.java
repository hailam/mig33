package javax.microedition.media.control;
import javax.microedition.media.Control;
import javax.microedition.media.MediaException;
public interface VideoControl extends Control {
 Object initDisplayMode(int mode,Object arg);
 void setVisible(boolean visible);
 void setDisplaySize(int w,int h) throws MediaException;
 void setDisplayLocation(int x,int y);
 void setDisplayFullScreen(boolean f) throws MediaException;
 int getSourceWidth(); int getSourceHeight();
 byte[] getSnapshot(String type) throws MediaException;
}
