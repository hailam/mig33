#!/usr/bin/env python3
"""Audit v4 endpoint literals, source anchors and original raw-field behavior.

First run verify-protocol-evidence.py to reproduce javap -c -p -s output.
This tool independently reads class-file constant pools, validates every stored
endpoint reference, and runs a bounded offline harness against the pinned
original ay/bj classes. No MIDlet or network connection is started. Semantic
claims below are manually reviewed; anchor checks are not a semantic proof.
"""

import argparse
from collections import Counter, defaultdict
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import struct
import subprocess
from zipfile import ZipFile


ROOT = Path(__file__).resolve().parents[3]
V4 = ROOT / "research/applets/v4"
SHA = "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c"
PREFIXES = ("http://", "https://", "socket://", "/sites/")
U_SEND = "public final synchronized ay[] a(ay, int);"
BC_RECEIVE = "public final synchronized void a(ay[]);"
AJ_COMMAND = "public final void commandAction(javax.microedition.lcdui.Command, javax.microedition.lcdui.Displayable);"

# Independent summaries of the manually traced argument-producing instructions.
# Keys select exact existing route literals; no expansion is inferred from a
# query parameter's English name.
ROUTE_REVIEWS = {
    "c=registration_api": "The username editor value in local22 is passed directly to y.b then bc.g (local1005), bypassing Q. This call does not perform component escaping; editor normalization is outside this route-argument check.",
    "c=im&a=setup_im": "Both call sites stringify the selected service object's bj.d byte as a decimal integer, call y.b, then y.e(result,null).",
    "c=sms": "Five sites: selected participant text; private peer s.c mobile if present or s.f username otherwise; roster t.d mobile; native-bridge argument. Every page call supplies null body.",
    "c=call": "Five sites: participant action106 refreshes current roster selection and reads t.d; private s.f username fallback or s.c mobile; roster t.d; native-bridge argument. Parameter name does not prove phone-number-only validation.",
    "c=profile&v=midlet&a=home&username": "Three sites use selected participant text, private s.f username and roster t.e username respectively. Participant action102 is confirmed by the command switch and local4 assignment.",
    "c=store&v=midlet&a=home&e=3&ty=1": "Three sites use participant text (action100), private s.f and roster t.e. The body argument is null.",
    "c=lookout": "Private route builds &cid= from s.e but &pres= from cached aq/t.j. Roster route uses selected t.a and t.j. Both substitute a complete suffix.",
    "c=account&v=midlet&a=transfer_credit&recipient": "Participant text, private s.f and roster t.c feed three sites. Roster t.c is the display-name field. Private/participant nonempty checks use object-reference comparison against the empty literal; the no-recipient route is separate.",
    "c=chatroom&v=midlet&a=setup": "Two sites read selected room-list bj.c; one reads current conversation s.b. They use the same template; no separate theme endpoint is constructed.",
    "c=group&v=midlet&a=add_to_group": "All three sites read current conversation s.b, replace through y.b, then resolve y.e(result,null).",
    "subject=Group%20Chat": "The Group Chat branch substitutes current conversation s.b with y.b, then resolves with y.e(result,null).",
    "subject=Chatroom": "The Chatroom branch substitutes current conversation s.b with y.b, then resolves with y.e(result,null).",
    "c=profile&a=user_blog_posts": "Reads saved profile/settings packet j.a text1, substitutes with y.b, then resolves y.e(result,null).",
    "c=email": "Compose Email passes the empty string; Send Email builds &to= plus selected t.e. Both call y.e and supply null body.",
    "c=photo&v=midlet&username": "My Photos reads j.a text1. View Photos prefers selected t.e and falls back to j.a text1. The504-response fallback passes null replacement, retaining%s whenever Q exists.",
    "c=help&a=contact_us": "Builds &to= plus saved j.a text1 and substitutes the whole suffix, with null page body.",
    "c=buzz": "Builds &cid= plus selected t.a and &pres= plus selected t.j, substituting the complete suffix.",
    "c=group&v=midlet&a=list_chatrooms": "Appends &cid= plus current conversation s.a after base resolution; null body.",
    "c=group&v=midlet&a=members": "Appends &cid= plus current conversation s.a after base resolution; null body.",
    "c=group&v=midlet&a=view_forum": "Appends &cid= plus current conversation s.a before base resolution; null body.",
    "c=group&v=midlet&a=home": "Appends &cid= plus current conversation s.a before base resolution; null body.",
    "c=location": "l(String) appends its raw argument after f=. Self-menu passes HOME; the checkIn native bridge passes its parsed argument.",
    "c=photo&v=midlet&a=home&f=sendfromphoto": "Initial navigation is selected when aO!=55, after saving tab type bl and conversation key bm (s.b for tab4; s.h otherwise). Null body; the sendScrapbookPhoto bridge uses those saved fields. Later media payload/UI behavior is outside this route check.",
    "c=photo&a=home&v=midlet": "Initial navigation is selected when aO==55, with null replacement and null body, and no added username query. Subsequent display-picture/UI transition behavior is outside this route check.",
    "sites/resources/language": "All three references call bc.b with the absolute URL; transport intercepts the resulting1001 local job and fetches it as a resource packet.",
    "c=help&v=wap&a=apn": "Both references invoke aj.k(String), which calls MIDlet.platformRequest; they do not construct922 here.",
    "c=help&v=wap&a=tc": "Invokes aj.k(String), which calls MIDlet.platformRequest; no embedded form body is supplied.",
    "sites/c=migworld": "All four references are in bl.c's local model/cache/history initialization. This method does not request the literal from HTTP.",
}


def source(class_name, method, *offsets):
    return (class_name, method, offsets)


# These describe the unchanged original client, not requirements to copy crashes
# or unsafe parsing into the Go server. Offsets are stable within the pinned JAR.
CLAIMS = [
    ("tcp-connection", "confirmed", "TCP uses socket:// + configured host + ':' + configured port, with separate input/output streams; normal writes serialize ay and flush, and the receiver repeatedly parses one ay frame from the stream.", [source("u", "public final synchronized void a(java.lang.String, java.lang.String);", 0, 26, 34, 42, 55), source("u", U_SEND, 1814), source("u", "public final void run();", 0, 232, 240)]),
    ("gateway-defaults", "confirmed", "Static defaults are HTTP http://gateway.mig33.com/, TCP gateway.mig33.com and port25; saved host/port settings can override these. The connection-test sequence toggles port25/9119 on the second TCP attempt.", [source("bc", "public static {};", 4, 9, 14), source("bc", "public final void run();", 646, 725)]),
    ("http-session-path", "confirmed", "The binary HTTP tunnel appends session text directly to the base for ordinary packets and null-request polls. Requests200/100/2/904 omit it. This is a path suffix, not a Bearer token or query encoding.", [source("u", U_SEND, 1078, 1095, 1105, 1114, 1122, 1134, 1157)]),
    ("binary-http", "confirmed", "Non-null tunnel requests POST serialized packet bytes with image/gif and byte-count Content-Length; a null request polls with GET. HTTP200 is required and the body is decoded as concatenated binary frames.", [source("u", U_SEND, 1213, 1239, 1255, 1317, 1348, 1445, 1488)]),
    ("direct-page-routing", "confirmed", "922 URLs starting with case-sensitive https use the private direct helper. Other922 requests can be direct in HTTP mode when caller mode2 is selected; otherwise they use the binary transport. The private helper synthesizes923 status200 on non-null bytes and404 otherwise, retaining transaction correlation.", [source("u", U_SEND, 0, 129, 188, 209)]),
    ("local-http-jobs", "confirmed", "1001/1003 and1005 are intercepted as local HTTP jobs before tunnel/TCP serialization. They do not establish three extra gateway operations.", [source("u", U_SEND, 129, 153, 1838)]),
    ("form-byte-length", "confirmed-original-quirk", "Both direct form paths use platform-default String.getBytes(). The parallel922 direct branch uses encoded byte length; the separate private helper advertises String.length() characters as Content-Length before writing encoded bytes.", [source("u", U_SEND, 277, 315, 331), source("u", "private synchronized byte[] b(java.lang.String, java.lang.String);", 108, 111, 140)]),
    ("http-header-profile", "confirmed", "The shared helper adds a header named by language string193, client version, sw/sh, cellid/lac/mnc/mcc, ver, saved lang, and epoch milliseconds. sid is conditional on authenticated bc.c; User-Agent on platform availability; Cookie on non-null assembled value, including empty; mode1 adds Pragma:no-cache.", [source("u", "private static void a(javax.microedition.io.HttpConnection, java.lang.String, int);", 1, 16, 40, 64, 75, 86, 97, 108, 119, 135, 158, 175, 244, 262)]),
    ("original-authentication", "confirmed", "Incoming201 tag2 sets session u.a; tag1 supplies the challenge. Client constructs202 tag1 as Java String.hashCode of challenge + saved password.toLowerCase(). This source verifies the client computation, not historical server acceptance or password policy.", [source("bc", BC_RECEIVE, 2585, 2592, 2623, 2634, 2660, 2669, 2672)]),
    ("transaction-numbering", "confirmed", "The synchronized counter is incremented before wire transaction assignment; ordinary queued requests and generated pings use (short)(counter %32767 +1). This is not an unrestricted unsigned16-bit counter.", [source("bc", "private int i();", 7, 14), source("bc", "public final void run();", 2102, 2211)]),
    ("idle-keepalive", "confirmed", "The scheduler compares idle time strictly greater than60000ms for TCP or5000ms for HTTP. TCP queues2 with field1 int1 if no ping is pending; an outstanding ping invokes reconnect. HTTP uses null-request GET polling. These are client scheduling thresholds, not server guarantees.", [source("bc", "public final void run();", 2048, 2076, 2138, 2181)]),
    ("timeouts", "confirmed", "The main HTTP timer is120s before authenticated flag and30s afterwards. The separate private HTTP/HTTPS helper schedules its own30s timeout. Connection-test wait is45s before authentication/60s after; request400 gets a120s wait in the scheduler.", [source("u", "private void h();", 30), source("u", "private synchronized byte[] b(java.lang.String, java.lang.String);", 35), source("bc", "public final void run();", 1204, 1232)]),
    ("parallel-http-queue", "confirmed", "Enabled flagq plus HTTP mode and existing h queue route922 through that separate queue. It is conditional client behavior, not an unconditional second HTTP API.", [source("bc", "private void b(ay);", 7, 56)]),
    ("cookie-host-selection", "confirmed-original-quirk", "Cookie lookup chooses a stored key by substring matching the extracted host; it does not implement standard domain/path scoping. Any URL containing img.mig33.com gets an empty cookie string.", [source("ac", "private static java.lang.String b(java.lang.String);", 0), source("ac", "public static synchronized java.lang.String a(java.lang.String);", 3)]),
    ("cookie-parsing", "confirmed-original-quirk", "Set-Cookie handling splits semicolon components, then key/value at the first equals sign, storing attributes as ordinary entries. The approximate1024-byte budget mixes byte sizing with character slicing. No browser-equivalent expiry/security semantics are established.", [source("ac", "public static synchronized java.lang.String a(java.lang.String, java.lang.String[]);", 17, 120)]),
    ("website-base", "confirmed", "203 tag26 sets aj.Q to field26.substring(0, field26.indexOf('/',7)+1). This is not a general URI parser. The route helper returns null whenever Q is null, even for an absolute input.", [source("bc", BC_RECEIVE, 13988, 14006, 14011, 14014), source("y", "public static java.lang.String e(java.lang.String, java.lang.String);", 0)]),
    ("asset-base", "confirmed", "203 tag23 is persisted as settings9. The two image helpers concatenate that base, image identifier, ?w=, &h= and &a=1&c=1; one uses explicit dimensions and one uses G twice. Identifiers starting with case-sensitive http pass through unchanged.", [source("bc", BC_RECEIVE, 13969, 13974, 13979), source("aj", "public static java.lang.String a(java.lang.String, int, int);", 1, 16, 31, 40, 49), source("aj", "public static java.lang.String d(java.lang.String);", 36, 47)]),
    ("url-resolution", "confirmed-original-quirk", "Relative page resolution joins slash boundaries against Q; generic replacement substitutes only the first literal%s. Null substitution keeps the marker. Generic URL escaping replaces only ASCII space with%20; these helpers are not standard query/form encoders.", [source("y", "public static java.lang.String e(java.lang.String, java.lang.String);", 0), source("y", "public static java.lang.String b(java.lang.String, java.lang.String);", 0), source("y", "public static java.lang.String a(java.lang.String, java.lang.String, java.lang.String);", 0), source("y", "public static java.lang.String c(java.lang.String, java.lang.String);", 0), source("y", "public static java.lang.String g(java.lang.String);", 0)]),
    ("wire-envelope", "confirmed", "Marker2, signed-short opcode, signed-short transaction, signed-int body length; fields have signed-short tag, signed-int payload length and raw payload. Java DataInput/DataOutput impose big-endian numeric byte order. No transmitted field-type discriminator exists.", [source("ay", "public final void a(java.io.InputStream);", 10, 19, 27, 45, 54, 61, 89), source("ay", "public final byte[] b();", 22, 31, 39, 47)]),
    ("field-order-duplicates", "confirmed-original-behavior", "ay stores fields in Hashtable under short tag, so later duplicates replace earlier values and serialization enumeration order is unspecified. Raw wire order must not become an application requirement.", [source("ay", "public final void a(bj);", 9, 16), source("ay", "public final byte[] b();", 54)]),
    ("numeric-prefix-reads", "confirmed-by-original-runtime", "A raw field can be read by several accessors:00010002 gives byte0, short1, int65538 and longnull. Numeric readers consume only their width from the start, ignoring trailing bytes. Mixed accessor use is therefore possible on the same wire bytes.", [source("bj", "public java.lang.Byte d();", 24, 30), source("bj", "public java.lang.Short g();", 0), source("bj", "public java.lang.Integer e();", 0), source("bj", "public java.lang.Long f();", 0)]),
    ("typed-versus-decoded-fields", "confirmed-by-original-runtime", "A locally constructed int field exposes only its matching typed getter, but serialization followed by original decoding produces raw bytes that expose numeric-prefix, text and array reads. Internal object discriminator must not be treated as a wire type code.", [source("bj", "public java.lang.Byte d();", 0), source("bj", "public java.lang.Integer e();", 0), source("ay", "public final void a(java.io.InputStream);", 89)]),
    ("missing-versus-empty", "confirmed-by-original-runtime", "Missing fields return null through ay accessors. A present zero-byte raw field returns empty text and empty array; short/int/long return null, but byte access throws ArrayIndexOutOfBoundsException. Safe clone accessors need not reproduce this malformed-input crash.", [source("ay", "public final java.lang.Byte b(short);", 7, 15), source("bj", "public java.lang.Byte d();", 30), source("bj", "public java.lang.String h();", 0)]),
    ("text-encoding", "confirmed", "Raw text is decoded with UTF-8, with fallback to platform-default constructor only on exception. Strings are not DataOutput.writeUTF strings. The Java21 probe replaces invalid ff with U+FFFD; that runtime observation alone does not prove every historical CLDC malformed-Unicode behavior.", [source("bj", "public java.lang.String h();", 0), source("bj", "public byte[] i();", 119, 125)]),
    ("string-array-loss", "confirmed-by-original-runtime", "TA serialization writes UTF-8 values with NUL only between values. Original decoding has a stale previous-segment-length loop bound and can drop valid nonempty later entries: a\\0b becomes[a]; ab\\0cd\\0ef becomes[ab,cd]; alice@example.test\\0bob becomes[alice@example.test]. The issue is broader than empty-segment handling.", [source("bj", "public byte[] i();", 72, 108, 110), source("bj", "public java.lang.String[] j();", 38, 43, 49, 52, 130)]),
    ("frame-length-trust", "confirmed-by-original-runtime", "The original parser accepts negative body length as an empty body and can consume a field extending beyond the declared body length. Negative field length throws NegativeArraySizeException; truncation throws EOFException. These are original validation defects, not requirements to weaken the bounded Go codec.", [source("ay", "public final void a(java.io.InputStream);", 49, 50, 61, 70, 72, 75)]),
    ("concatenation", "confirmed-by-original-runtime", "ay.a(byte[]) parses until the input is exhausted and returns null for an empty stream. Two complete concatenated frames become two packets. An invalid marker or trailing truncated frame throws rather than returning a successful prefix batch.", [source("ay", "public static ay[] a(byte[]);", 17, 34, 62), source("ay", "public final void a(java.io.InputStream);", 99)]),
    ("participant-route-source", "confirmed", "Participant-menu value comes from current selection y.a(y.m), stored in local4. Switch actions100/102/108/1000 use it for gift/profile/SMS/transfer respectively; action106 instead refreshes the roster selection before reading its mobile.", [source("aj", AJ_COMMAND, 684, 687, 734, 955, 990, 1020, 1039, 1208, 1279)]),
    ("dynamic-conversation-menu", "confirmed", "Server conversation-menu prefix gets imType=s.g()&cid=s.e() for private, gcid=s.b for group and roomName=s.b for room. No separator is inserted between prefix and the first key.", [source("aj", AJ_COMMAND, 3450, 3457, 3470, 3488, 3513, 3536, 3545)]),
    ("dynamic-contact-menu", "confirmed", "Overlay1 appends cid=selected t.a&pres=selected t.j, overlay2 appends gid=selected t.b, other custom menu URLs pass through. Prefix must supply its own initial query separator.", [source("aj", AJ_COMMAND, 4380, 4418, 4427, 4442, 4461, 4499, 4508, 4525)]),
    ("dynamic-notification-menu", "confirmed", "Notification action opens t.u; Read Email opens aj.P; Updates ClickLink passes aj.ac (an av instance) h() result to the page/native bridge. They do not acquire the contact/group-menu suffix automatically.", [source("aj", AJ_COMMAND, 8803, 9299, 9350, 9355)]),
    ("form-method-and-escaping", "confirmed", "The HTML renderer defaults form method to get, lowercases supplied method, builds control values in page order and substitutes spaces with%20. get makes action+'?'+body; other methods use a separate body. This verifies client construction, not remotely supplied actions or backend validation.", [source("s", "public synchronized void b(javax.microedition.lcdui.Graphics);", 3898, 3925), source("s", "public synchronized java.lang.String[] b();", 434, 960, 963, 980, 1006, 1022)]),
    ("navigation-escaping", "confirmed", "Normal aj page opening runs y.g(space replacement) on URLs not starting P, then delegates URL and body through bl. The route literals' null bodies therefore express initial GET navigation, not the eventual method of a returned form.", [source("aj", "public final void a(java.lang.String, java.lang.String, boolean, boolean, bl);", 14, 23, 122, 127)]),
]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def constant_strings(data):
    """Read original CONSTANT_String values without relying on javap text."""
    assert data[:4] == b"\xca\xfe\xba\xbe"
    size = struct.unpack_from(">H", data, 8)[0]
    position, index = 10, 1
    values, references = {}, []
    widths = {3: 4, 4: 4, 5: 8, 6: 8, 7: 2, 9: 4, 10: 4, 11: 4,
              12: 4, 15: 3, 16: 2, 17: 4, 18: 4, 19: 2, 20: 2}
    while index < size:
        tag = data[position]
        position += 1
        if tag == 1:
            length = struct.unpack_from(">H", data, position)[0]
            position += 2
            # All endpoint candidates are ASCII. Leave other modified-UTF8
            # strings as bytes; do not mislabel UTF8 replacement as recovery.
            values[index] = data[position:position + length]
            position += length
        elif tag == 8:
            references.append(struct.unpack_from(">H", data, position)[0])
            position += 2
        else:
            position += widths[tag]
            if tag in (5, 6):
                index += 1
        index += 1
    result = []
    for reference in references:
        raw = values[reference]
        if any(raw.startswith(prefix.encode("ascii")) for prefix in PREFIXES):
            result.append(raw.decode("ascii"))
    return result


def parse_dump(path):
    methods, literal_refs = {}, []
    current = None
    for number, line in enumerate(path.read_text().splitlines(), 1):
        if re.match(r"^  \S.*\(.*\);$", line) or re.match(r"^  (?:public )?static \{\};$", line):
            current = {"method": line.strip(), "descriptor": None, "instructions": {}}
            methods[current["method"]] = current
        elif current is not None and line.startswith("    descriptor:"):
            current["descriptor"] = line.split(": ", 1)[1]
        instruction = re.match(r"\s+(\d+): (.*)", line)
        if instruction and current is not None:
            current["instructions"][int(instruction[1])] = {"line": number, "instruction": instruction[2]}
            literal = re.search(r"// String (.*)$", line)
            if literal and literal[1].startswith(PREFIXES):
                literal_refs.append({"literal": literal[1], "class": path.stem,
                    "method": current["method"], "descriptor": current["descriptor"],
                    "bytecode_offset": int(instruction[1]), "path": str(path.relative_to(ROOT)), "line": number})
    return methods, literal_refs


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--java-home")
    parser.add_argument("--write-catalog", action="store_true", help="Update tracked audit JSON after review")
    args = parser.parse_args()
    jar = V4 / "artifacts/original/mig33-4.62.jar"
    if digest(jar) != SHA:
        raise RuntimeError("Original JAR checksum mismatch")
    output = ROOT / "data/protocol-audit"
    output.mkdir(parents=True, exist_ok=True)
    original_literals = defaultdict(set)
    with ZipFile(jar) as archive:
        classes = sorted(name[:-6].replace("/", ".") for name in archive.namelist() if name.endswith(".class"))
        for name in classes:
            for literal in constant_strings(archive.read(name.replace(".", "/") + ".class")):
                original_literals[literal].add(name)
        manifest = archive.read("META-INF/MANIFEST.MF").decode("ascii")

    methods, observed, dumps = {}, [], []
    for name in classes:
        path = V4 / f"artifacts/bytecode/{name}.txt"
        methods[name], references = parse_dump(path)
        observed.extend(references)
        dumps.append({"class": name, "sha256": digest(path)})
    catalog_path = V4 / "docs/data/protocol-endpoints.json"
    catalog = json.loads(catalog_path.read_text())
    classified_path = V4 / "docs/data/endpoints-classified.json"
    classified = json.loads(classified_path.read_text())
    expected = {(row["literal"], ref["path"], ref["line"])
                for row in catalog for ref in row["references"]}
    actual = {(ref["literal"], ref["path"], ref["line"]) for ref in observed}
    literal_set = {row["literal"] for row in catalog}
    counts = Counter(ref["literal"] for ref in observed)
    checks = {
        "original_classes_scanned": len(classes), "catalog_literals": len(catalog),
        "original_constant_pool_literals": len(original_literals), "bytecode_literal_occurrences": len(observed),
        "catalog_reference_count": sum(len(row["references"]) for row in catalog),
        "all_literal_sets_equal": literal_set == set(original_literals) == set(counts),
        "all_reference_lines_match": expected == actual,
        "missing_references": [list(ref) for ref in sorted(expected - actual)],
        "extra_references": [list(ref) for ref in sorted(actual - expected)],
        "occurrences_by_class": dict(sorted(Counter(ref["class"] for ref in observed).items())),
        "classification_counts": {"relative_page_templates": sum(x.startswith("/sites/") for x in literal_set),
            "scheme_only": sum(x in PREFIXES[:3] for x in literal_set),
            "absolute_url_or_url_prefix": sum(x.startswith(("http://", "https://", "socket://")) and x not in PREFIXES for x in literal_set)},
        "scope": "Exact ASCII CONSTANT_String values and executable ldc references beginning http://, https://, socket:// or /sites/; not all assembled/server-supplied routes or server validation semantics.",
    }
    classified_refs = {(row["literal"], ref["path"], ref["line"])
        for row in classified["literals"] + classified["excluded"] for ref in row["references"]}
    checks["classified_catalog"] = {
        "sha256": digest(classified_path),
        "url_path_literals": len(classified["literals"]), "excluded_scheme_literals": len(classified["excluded"]),
        "constructed_or_descriptor_entries": len(classified["constructed_and_descriptor_urls"]),
        "all_original_literal_references_preserved": classified_refs == expected,
        "scope": "Classified59 plus excluded3 are a lossless partition of raw inventory references. Constructed/descriptor semantics are covered by source claims and installation_evidence; absent JAD remains unverified.",
    }
    claims = []
    for identifier, status, statement, sources in CLAIMS:
        evidence = []
        for class_name, method, offsets in sources:
            body = methods[class_name][method]
            anchors = []
            for offset in offsets:
                if offset not in body["instructions"]:
                    raise RuntimeError(f"Missing source anchor {class_name}.{method} @{offset}")
                anchors.append({"offset": offset, **body["instructions"][offset]})
            evidence.append({"class": class_name, "method": method, "descriptor": body["descriptor"], "anchors": anchors})
        claims.append({"id": identifier, "status": status, "statement": statement, "evidence": evidence})

    route_path = V4 / "docs/data/endpoint-route-notes.json"
    route_notes = json.loads(route_path.read_text())
    ref_lookup = {(ref["literal"], ref["path"], ref["line"]): ref for ref in observed}
    route_reviews = []
    for literal, route in route_notes["routes"].items():
        matches = [review for key, review in ROUTE_REVIEWS.items() if key in literal]
        if len(matches) != 1:
            raise RuntimeError(f"Missing or ambiguous manual route review: {literal}")
        evidence = []
        for reference in route["original_bytecode_refs"]:
            path, line = reference.rsplit(":", 1)
            ref = ref_lookup[(literal, path, int(line))]
            evidence.append({key: value for key, value in ref.items() if key not in ("literal", "path")})
        route_reviews.append({"literal": literal, "status": "source-reviewed",
            "verified_scope": matches[0], "evidence": evidence})
    placeholder_reviews = [row for row in route_reviews if "%s" in row["literal"]]

    spec = importlib.util.spec_from_file_location("verify_evidence", ROOT / "tools/applets/v4/verify-protocol-evidence.py")
    helper = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(helper)
    java, javac = (helper.java_tool(name, args.java_home) for name in ("java", "javac"))
    emulator = ROOT / "tools/applets/bin/microemulator-2.0.4.jar"
    tool_manifest = json.loads((ROOT / "tools/applets/bin/manifest.json").read_text())
    expected_emulator = next(item for item in tool_manifest["tools"] if item["file"] == emulator.name)
    if digest(emulator) != expected_emulator["sha256"]:
        raise RuntimeError("MicroEmulator checksum mismatch")
    harness = ROOT / "tools/applets/v4/protocol-accessor-audit.java"
    build = output / "accessor-build"
    build.mkdir(parents=True, exist_ok=True)
    classpath = os.pathsep.join(map(str, (jar, emulator)))
    subprocess.run([javac, "-cp", classpath, "-d", str(build), str(harness)], check=True, cwd=ROOT)
    probe_text = subprocess.run([java, "-Djava.awt.headless=true", "-cp", str(build) + os.pathsep + classpath,
        "ProtocolAccessorAudit"], check=True, capture_output=True, text=True, cwd=ROOT).stdout
    (output / "accessors.jsonl").write_text(probe_text)
    probes = [json.loads(line) for line in probe_text.splitlines()]
    for probe in probes:
        if "long" in probe:
            value = probe.pop("long")
            probe["long_decimal"] = str(value) if isinstance(value, int) else value
    by_name = {probe["name"]: probe for probe in probes}
    probe_checks = {
        "same_raw_bytes_multiple_numeric_reads": all(by_name["raw_00010002"][key] == value for key, value in {"byte": 0, "short": 1, "int": 65538, "long_decimal": None}.items()),
        "empty_present_byte_throws": by_name["raw_"]["byte"] == {"throws": "java.lang.ArrayIndexOutOfBoundsException"},
        "array_drops_valid_final_entry": by_name["array_610062"]["text_array"] == ["a"],
        "array_retains_longer_final_entry": by_name["array_61006263"]["text_array"] == ["a", "bc"],
        "array_original_fixture_loses_bob": by_name["array_616c696365406578616d706c652e7465737400626f62"]["text_array"] == ["alice@example.test"],
        "duplicate_fields_last_wins": by_name["duplicate_field_last_wins"]["packets"][0]["field_1_text"] == "b",
        "negative_body_accepted": len(by_name["negative_body_length"].get("packets", [])) == 1,
        "body_length_overrun_accepted": by_name["body_shorter_than_field"]["packets"][0]["field_1_text"] == "a",
        "negative_field_length_throws": by_name["negative_field_length"].get("throws") == "java.lang.NegativeArraySizeException",
        "two_concatenated_frames": len(by_name["two_frames"]["packets"]) == 2,
    }
    report = {
        "version": "4.62", "original_jar_sha256": SHA,
        "scope": "Original-client endpoint inventory, transport/session helpers and shared field/frame semantics. Offline execution of original ay/bj only. Manually reviewed semantic statements are distinct from automated anchor and inventory checks.",
        "reproduce": "python3 tools/applets/v4/verify-protocol-evidence.py && python3 tools/applets/v4/audit-transports.py --write-catalog",
        "source": {"javap_arguments": ["-c", "-p", "-s"], "catalog_sha256": digest(catalog_path),
            "endpoint_route_notes_sha256": digest(route_path), "endpoints_classified_sha256": digest(classified_path), "disassembly_sha256": dumps,
            "harness": str(harness.relative_to(ROOT)), "harness_sha256": digest(harness),
            "java_version": subprocess.run([java, "-version"], capture_output=True, text=True, check=True).stderr.strip()},
        "endpoint_inventory": checks,
        "endpoint_literals": [{"literal": literal, "original_constant_pool_classes": sorted(original_literals[literal]),
            "verified_bytecode_occurrences": counts[literal]} for literal in sorted(literal_set)],
        "route_construction_audit": {
            "input_sha256": digest(route_path), "reviewed_route_entries": len(route_reviews),
            "placeholder_templates": len(placeholder_reviews),
            "relative_placeholder_templates": sum(row["literal"].startswith("/sites/") for row in placeholder_reviews),
            "absolute_placeholder_templates": sum(row["literal"].startswith("http") for row in placeholder_reviews),
            "placeholder_call_sites": sum(len(row["evidence"]) for row in placeholder_reviews),
            "reviewed_literal_call_sites": sum(len(row["evidence"]) for row in route_reviews),
            "initial_scope_wording_correction": "Initial common.scope said17 literal /sites/ templates; actual count is17 total:16 relative and1 absolute registration URL. This audit records the correction even after integration into the input.",
            "dynamic_routes": "All four documented categories rechecked: conversation-menu suffix, contact/group-menu suffix, plain notification/mail/update URL, and avatar/image construction; see claims.",
            "entries": route_reviews,
            "remaining_semantic_limits": ["Username editor normalization is outside the registration URL argument check.", "The two From My Photo route entries are checked through initial navigation and saved conversation context; subsequent complete media/UI flows are outside this route audit.", "Actual server-supplied form actions, content, authorization and validation rules cannot be reconstructed from these client call sites."],
        },
        "installation_evidence": {"manifest_install_notify": next(line.split(": ", 1)[1] for line in manifest.splitlines() if line.startswith("MIDlet-Install-Notify:")),
            "jad_distribution_url": {"status": "not-rechecked-source-unavailable", "path": "research/applets/v4/artifacts/original/mig33-4.62.jad",
                "reason": "The local JAD is absent; the documented distribution URL is historical catalog evidence, not independently verified by this run."}},
        "claims": claims, "original_runtime_probe_count": len(probes), "original_runtime_probes": probes,
        "original_runtime_checks": probe_checks,
        "limits": [
            "All28 documented literal route entries are checked for initial navigation/construction; all17 placeholder templates and41 call sites are traced. Explicit remaining semantic limits are recorded in route_construction_audit.",
            "No live legacy host, login, billing, moderation, room policy or deployment behavior is inferred.",
            "The v4 original JAR is the authority; transformed analysis-only JARs and clone behavior are not used as original-runtime proof.",
            "Original malformed-length and zero-byte crashes are documented separately from safe clone codec validation.",
            "Java21 runtime confirms these bounded probes; platform-default charset and Unicode edge cases may differ on historical CLDC devices.",
        ],
    }
    report["passed"] = checks["all_literal_sets_equal"] and checks["all_reference_lines_match"] and checks["classified_catalog"]["all_original_literal_references_preserved"] and all(probe_checks.values())
    destination = V4 / "docs/data/protocol-audit-transports.json" if args.write_catalog else output / "transports.json"
    destination.write_text(json.dumps(report, indent=2, ensure_ascii=True) + "\n")
    (output / "endpoint-reference-anchors.json").write_text(json.dumps(observed, indent=2) + "\n")
    print(f"Endpoint literals: {len(literal_set)}; original bytecode references: {len(observed)}; source claims: {len(claims)}; original runtime probes: {len(probes)}")
    print(f"Passed: {report['passed']}; report: {destination.relative_to(ROOT)}")
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
