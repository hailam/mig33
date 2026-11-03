import AppKit
import Foundation

struct Point: Hashable { let x: Int; let y: Int }
let source = URL(fileURLWithPath: CommandLine.arguments[1])
let bitmap = NSBitmapImageRep(data: try Data(contentsOf: source))!
let w = bitmap.pixelsWide, h = bitmap.pixelsHigh
var cyan = [Bool](repeating: false, count: w*h)
var orange = cyan, white = cyan
for y in 0..<h { for x in 0..<w {
  let color = bitmap.colorAt(x: x, y: y)!.usingColorSpace(.deviceRGB)!
  let r = color.redComponent*255, g = color.greenComponent*255, b = color.blueComponent*255
  guard color.alphaComponent >= 0.5 else { continue }
  let i = y*w+x
  cyan[i] = g > r+20 && b > r+20
  orange[i] = r > g+20 && r > b+40
  white[i] = min(r,g,b) > 220 && max(r,g,b)-min(r,g,b) < 20
}}
func fillHoles(_ mask: [Bool]) -> [Bool] {
  var outside = [Bool](repeating:false,count:w*h), queue = [Int]()
  for y in 0..<h { for x in 0..<w where x == 0 || y == 0 || x == w-1 || y == h-1 {
    let i=y*w+x; if !mask[i] && !outside[i] { outside[i]=true; queue.append(i) }
  }}
  var n=0
  while n<queue.count {
    let i=queue[n]; n+=1; let x=i%w,y=i/w
    for (xx,yy) in [(x-1,y),(x+1,y),(x,y-1),(x,y+1)] where xx>=0 && xx<w && yy>=0 && yy<h {
      let j=yy*w+xx; if !mask[j] && !outside[j] { outside[j]=true;queue.append(j) }
    }
  }
  return (0..<w*h).map { mask[$0] || !outside[$0] }
}
cyan=fillHoles(cyan);orange=fillHoles(orange)
func contours(_ mask:[Bool]) -> [[Point]] {
  var edges=[Point:[Point]]()
  func on(_ x:Int,_ y:Int)->Bool { x>=0 && y>=0 && x<w && y<h && mask[y*w+x] }
  func edge(_ x:Int,_ y:Int,_ xx:Int,_ yy:Int) { edges[Point(x:x,y:y),default:[]].append(Point(x:xx,y:yy)) }
  for y in 0..<h { for x in 0..<w where on(x,y) {
    if !on(x,y-1){edge(x,y,x+1,y)};if !on(x+1,y){edge(x+1,y,x+1,y+1)}
    if !on(x,y+1){edge(x+1,y+1,x,y+1)};if !on(x-1,y){edge(x,y+1,x,y)}
  }}
  func direction(_ a:Point,_ b:Point)->Int { b.x>a.x ? 0 : b.y>a.y ? 1 : b.x<a.x ? 2 : 3 }
  var output=[[Point]]()
  while let start=edges.keys.sorted(by:{$0.y == $1.y ? $0.x<$1.x : $0.y<$1.y}).first {
    var path=[start],current=start,previousDirection=0
    repeat {
      guard let options=edges[current],!options.isEmpty else { fatalError("Open contour") }
      let next=options.min { a,b in
        func rank(_ p:Point)->Int { let turn=(direction(current,p)-previousDirection+4)%4; return turn == 1 ? 0 : turn == 0 ? 1 : turn == 3 ? 2 : 3 }
        return rank(a)<rank(b)
      }!
      edges[current]!.removeAll{$0==next};if edges[current]!.isEmpty{edges.removeValue(forKey:current)}
      previousDirection=direction(current,next);current=next;path.append(next)
    } while current != start
    // Keep a deterministic source polygon for fitting and deviation checks.
    path.removeLast()
    let compact=path.indices.filter { i in
      let a=path[(i+path.count-1)%path.count],b=path[i],c=path[(i+1)%path.count]
      return (b.x-a.x)*(c.y-b.y) != (b.y-a.y)*(c.x-b.x)
    }.map{path[$0]}
    if compact.count>=3 { output.append(compact) }
  }
  return output
}
struct Vector {
  var x: Double; var y: Double
  static func +(a: Self, b: Self) -> Self { .init(x: a.x+b.x, y: a.y+b.y) }
  static func -(a: Self, b: Self) -> Self { .init(x: a.x-b.x, y: a.y-b.y) }
  static func *(a: Self, b: Double) -> Self { .init(x: a.x*b, y: a.y*b) }
  func dot(_ b: Self) -> Double { x*b.x+y*b.y }
  var length: Double { hypot(x,y) }
  var unit: Self { self * (1 / max(0.000001, length)) }
}
struct Cubic {
  var a: Vector; var b: Vector; var c: Vector; var d: Vector
  func at(_ t: Double) -> Vector {
    let s = 1-t
    return a*(s*s*s) + b*(3*s*s*t) + c*(3*s*t*t) + d*(t*t*t)
  }
  func derivative(_ t: Double) -> Vector {
    (b-a)*(3*(1-t)*(1-t)) + (c-b)*(6*(1-t)*t) + (d-c)*(3*t*t)
  }
  func secondDerivative(_ t: Double) -> Vector { (c-b*2+a)*(6*(1-t)) + (d-c*2+b)*(6*t) }
}
func distance(_ point: Vector, _ a: Vector, _ b: Vector) -> Double {
  let edge=b-a, t=max(0,min(1,(point-a).dot(edge)/max(0.000001,edge.dot(edge))))
  return (point-(a+edge*t)).length
}
func flatten(_ curve: Cubic, tolerance: Double = 0.01) -> [Vector] {
  if max(distance(curve.b,curve.a,curve.d),distance(curve.c,curve.a,curve.d)) <= tolerance {
    return [curve.a,curve.d]
  }
  let ab=(curve.a+curve.b)*0.5,bc=(curve.b+curve.c)*0.5,cd=(curve.c+curve.d)*0.5
  let abc=(ab+bc)*0.5,bcd=(bc+cd)*0.5,m=(abc+bcd)*0.5
  return Array(flatten(Cubic(a:curve.a,b:ab,c:abc,d:m),tolerance:tolerance).dropLast())
    + flatten(Cubic(a:m,b:bcd,c:cd,d:curve.d),tolerance:tolerance)
}
func directedDeviation(_ source: [Vector], _ target: [Vector], step: Double = 0.05) -> Double {
  var result=0.0
  for index in 0..<(source.count-1) {
    let a=source[index], b=source[index+1], count=max(1,Int(ceil((b-a).length/step)))
    for n in 0...count {
      let point=a+(b-a)*(Double(n)/Double(count))
      var nearest=Double.infinity
      for j in 0..<(target.count-1) { nearest=min(nearest,distance(point,target[j],target[j+1])) }
      result=max(result,nearest)
    }
  }
  return result
}
func fit(_ points: [Vector], _ reference: [Vector], _ left: Vector, _ right: Vector,
  errorLimit: Double, deviationLimit: Double) -> [Cubic] {
  let first=points[0],last=points.last!
  if points.count == 2 {
    return [Cubic(a:first,b:first+(last-first)*(1/3),c:first+(last-first)*(2/3),d:last)]
  }
  var parameters=[0.0]
  for i in 1..<points.count { parameters.append(parameters.last!+(points[i]-points[i-1]).length) }
  let total=parameters.last!
  parameters=parameters.map{$0/total}
  func generate() -> Cubic {
    var c00=0.0,c01=0.0,c11=0.0,x0=0.0,x1=0.0
    for i in points.indices {
      let t=parameters[i],s=1-t,b0=s*s*s,b1=3*s*s*t,b2=3*s*t*t,b3=t*t*t
      let a1=left*b1,a2=right*b2,residual=points[i]-first*(b0+b1)-last*(b2+b3)
      c00+=a1.dot(a1);c01+=a1.dot(a2);c11+=a2.dot(a2);x0+=a1.dot(residual);x1+=a2.dot(residual)
    }
    let determinant=c00*c11-c01*c01
    var alpha=abs(determinant)>0.000001 ? (x0*c11-x1*c01)/determinant : 0
    var beta=abs(determinant)>0.000001 ? (c00*x1-c01*x0)/determinant : 0
    if alpha<0.00001 || beta<0.00001 || alpha>total*2 || beta>total*2 { alpha=(last-first).length/3;beta=alpha }
    return Cubic(a:first,b:first+left*alpha,c:last+right*beta,d:last)
  }
  var split=points.count/2
  for _ in 0..<5 {
    let curve=generate()
    // A locally reversing handle can satisfy distance bounds while creating
    // a tiny loop. Monotone control projections make that impossible; reject
    // it and subdivide instead of accepting a topologically invalid fit.
    let chord=curve.d-curve.a,extent=chord.dot(chord)
    let firstProjection=(curve.b-curve.a).dot(chord),secondProjection=(curve.c-curve.a).dot(chord)
    let monotone=firstProjection >= -0.000001 && secondProjection >= firstProjection
      && secondProjection <= extent+0.000001
    var error=0.0
    for i in 1..<(points.count-1) {
      let current=(curve.at(parameters[i])-points[i]).length
      if current>error { error=current;split=i }
    }
    if error <= errorLimit && monotone {
      let flattened=flatten(curve)
      let deviation=max(directedDeviation(reference,flattened),directedDeviation(flattened,reference))
      if deviation <= deviationLimit { return [curve] }
    }
    if error>1.5 { break }
    var next=parameters
    for i in 1..<(points.count-1) {
      let t=parameters[i],delta=curve.at(t)-points[i],firstDerivative=curve.derivative(t)
      let denominator=firstDerivative.dot(firstDerivative)+delta.dot(curve.secondDerivative(t))
      if abs(denominator)>0.000001 {
        next[i]=max(parameters[i-1]+0.000001,min(parameters[i+1]-0.000001,t-delta.dot(firstDerivative)/denominator))
      }
    }
    parameters=next
  }
  // Shared tangents keep adjacent fitted pieces smooth. At an actual source
  // corner the bounded fit naturally subdivides rather than rounding it away.
  let tangent=(points[min(points.count-1,split+2)]-points[max(0,split-2)]).unit
  return fit(Array(points[...split]),Array(reference[...split]),left,tangent * -1,
      errorLimit: errorLimit, deviationLimit: deviationLimit)
    + fit(Array(points[split...]),Array(reference[split...]),tangent,right,
      errorLimit: errorLimit, deviationLimit: deviationLimit)
}
func smooth(_ polygon: [Point], body: Bool) -> ([Cubic],Double) {
  var points=[Vector]()
  for i in polygon.indices {
    let a=polygon[i],b=polygon[(i+1)%polygon.count]
    let length=max(abs(b.x-a.x),abs(b.y-a.y))
    for n in 0..<length {
      let t=Double(n)/Double(length)
      points.append(Vector(x:Double(a.x)+Double(b.x-a.x)*t,y:Double(a.y)+Double(b.y-a.y)*t))
    }
  }
  let original=points+[points[0]]
  // Glass highlights magnify one-pixel raster steps on the two body rims.
  // Their wider conditioning kernel has a separate one-pixel fidelity budget;
  // lettering retains its original kernel, limits, counters and fitted paths.
  // Both are checked against the untouched source polygon after fitting.
  let reference=points
  let weights: [Double] = body ? [1,4,7,10,12,10,7,4,1] : [1,2,4,2,1]
  let radius=weights.count/2, totalWeight=weights.reduce(0,+)
  let sampleLimit=body ? 0.75 : 0.35, deviationLimit=body ? 0.95 : 0.45
  points=points.indices.map { i in
    var sum=Vector(x:0,y:0)
    for offset in weights.indices {
      sum=sum+reference[(i+points.count+offset-radius)%points.count]*weights[offset]
    }
    let average=sum*(1/totalWeight)
    let delta=average-reference[i]
    return reference[i]+delta*min(1,sampleLimit/max(0.000001,delta.length))
  }
  // Four deterministic seams make closed loops fit without coincident-end
  // degeneracy. Every original loop (including letter counters) stays separate.
  let seams=[0,points.count/4,points.count/2,points.count*3/4,points.count]
  func tangent(_ i:Int)->Vector { (points[(i+3)%points.count]-points[(i+points.count-3)%points.count]).unit }
  var curves=[Cubic]()
  for n in 0..<4 {
    let start=seams[n],end=seams[n+1]
    curves+=fit((start...end).map{points[$0%points.count]},(start...end).map{reference[$0%points.count]},
      tangent(start),tangent(end%points.count) * -1,
      errorLimit: body ? 0.6 : 0.35, deviationLimit: deviationLimit)
  }
  let outline=curves.flatMap{Array(flatten($0).dropLast())}+[curves[0].a]
  let deviation=max(directedDeviation(original,outline),directedDeviation(outline,original))
  precondition(deviation <= deviationLimit+0.000001,"Contour exceeded its fitted deviation bound")
  func area(_ p:[Vector])->Double { zip(p,p.dropFirst()).reduce(0){$0+$1.0.x*$1.1.y-$1.1.x*$1.0.y} }
  precondition(area(original)*area(outline)>0,"Contour winding changed")
  return (curves,deviation)
}
func number(_ value:Double)->String { String(format:"%.5f",value) }
let masks:[(String,[Bool])]=[("cyan",cyan),("orange",orange),("mig",white.enumerated().map{cyan[$0.offset] && $0.element}),("number",white.enumerated().map{orange[$0.offset] && $0.element})]
print("import SwiftUI\n\n// Source-derived contours: bodies bounded to1px, lettering to0.5px; see PROVENANCE.md.\nenum Mig33LogoPaths {")
for (name,mask) in masks {
  let paths=contours(mask)
  if name == "cyan" || name == "orange" {
    let points=paths.flatMap{$0},left=points.map{$0.x}.min()!,right=points.map{$0.x}.max()!
    let top=points.map{$0.y}.min()!,bottom=points.map{$0.y}.max()!
    print("  static let \(name)Bounds = CGRect(x: \(left), y: \(top), width: \(right-left), height: \(bottom-top))")
  }
  print("  static let \(name): Path = make([")
  var worst=0.0,segments=0
  for path in paths {
    let (curves,deviation)=smooth(path,body:name == "cyan" || name == "orange");worst=max(worst,deviation);segments+=curves.count
    var values=[curves[0].a.x,curves[0].a.y]
    for curve in curves { values += [curve.b.x,curve.b.y,curve.c.x,curve.c.y,curve.d.x,curve.d.y] }
    print("    ["+values.map(number).joined(separator:",")+"],")
  }
  print("  ])")
  FileHandle.standardError.write(Data("\(name):\(paths.count)contours,\(segments)cubics,max sampled deviation\(number(worst))px\n".utf8))
}
print("""
  private static func make(_ contours: [[CGFloat]]) -> Path {
    var path = Path()
    for contour in contours {
      path.move(to: CGPoint(x: contour[0], y: contour[1]))
      for i in stride(from: 2, to: contour.count, by: 6) {
        path.addCurve(to: CGPoint(x: contour[i + 4], y: contour[i + 5]),
          control1: CGPoint(x: contour[i], y: contour[i + 1]),
          control2: CGPoint(x: contour[i + 2], y: contour[i + 3]))
      }
      path.closeSubpath()
    }
    return path
  }
}
""")
