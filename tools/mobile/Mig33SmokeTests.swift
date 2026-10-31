import XCTest

/// Exercises the real React Native controls and native networking modules.
/// Backend listeners and the Bob echo peer belong to the isolated runner.
@MainActor
final class Mig33SmokeTests: XCTestCase {
    private let app = XCUIApplication(bundleIdentifier: "com.hailam.mig33")
    private let environment = ProcessInfo.processInfo.environment
    private enum SystemAccessibility: String {
        case motion = "Reduce Motion"
        case transparency = "Reduce Transparency"
        case contrast = "Increase Contrast"
        var section: String { self == .motion ? "Motion" : "Display & Text Size" }
    }
    private var accessibilityOriginal: [SystemAccessibility: Bool] = [:]
    private var accessibilityNeedsRestore: Set<SystemAccessibility> = []

    override func setUpWithError() throws {
        continueAfterFailure = false
        try relaunch()
    }

    private func relaunch() throws {
        app.terminate()
        let link = try XCTUnwrap(environment["MIG33_SMOKE_DEEP_LINK"])
        app.open(try XCTUnwrap(URL(string: link)))
    }

    override func tearDownWithError() throws {
        if let run = testRun, run.totalFailureCount > 0 {
            capture("failure-screen")
            captureTree("failure-accessibility-tree")
        }
    }

    private func element(_ id: String) -> XCUIElement {
        app.descendants(matching: .any).matching(identifier: id).firstMatch
    }

    private func wait(_ id: String, timeout: TimeInterval = 20) -> XCUIElement {
        if ["login.auto", "login.invisible", "login.connection", "appearance.open"].contains(id),
           element("login.menu").exists, !menuControl(id).exists {
            tap("login.menu")
        }
        var result = menuControl(id)
        if !result.exists && nativeMenuLabels[id] != nil {
            revealNativeMenuAction(id)
            result = menuControl(id)
        }
        if !result.exists {
            let available = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                result = self.menuControl(id)
                return result.exists
            }, object: nil)
            XCTAssertEqual(XCTWaiter.wait(for: [available], timeout: timeout), .completed, "Missing control: \(id)")
        }
        return result
    }

    private func menuControl(_ id: String) -> XCUIElement {
        let identified = element(id)
        if identified.exists { return identified }
        // UIKit's scrollable SwiftUI Menu presentation drops action identifiers
        // while retaining exact accessible labels. Restrict this fallback to
        // buttons in an actual visible native menu, never matching app content.
        guard let expected = nativeMenuLabels[id] else { return identified }
        for container in app.menus.allElementsBoundByIndex + app.collectionViews.allElementsBoundByIndex
            where container.exists && container.isHittable {
            let matches = container.buttons.matching(NSPredicate(format: "label IN %@", expected))
            if matches.count == 1 { return matches.firstMatch }
        }
        return identified
    }

    private let nativeMenuLabels: [String: [String]] = [
            "chat.action.block": ["Block messages"], "chat.action.blocks": ["Blocked Users"],
            "chat.action.invite": ["Add to Chat"], "chat.action.kick": ["Kick User"],
            "chat.action.private": ["Private Chat"], "chat.menu.moderation": ["Moderation"],
            "chat.action.mute": ["Mute / unmute in this room"], "chat.action.info": ["Room Info"],
            "room.participants": ["Participants"], "group.participants": ["Participants"],
            "room.favorite": ["Add to Favorites", "Remove from Favorites"],
            "room.leave": ["Leave Room"], "group.leave": ["Leave Group Chat"],
            "room.join": ["Join Room", "Open Room"],
            "rooms.create": ["Create Room"], "rooms.search": ["Search Rooms"],
            "menu.rooms": ["Rooms"], "menu.exit": ["Log out"], "appearance.open": ["Appearance"],
            "login.connection": ["Connection Settings"], "login.auto": ["Auto Login"],
            "login.invisible": ["Login As Invisible"],
            "composer.open.emoticons": ["Insert Emoticon"], "composer.open.colors": ["Change Text Color"],
    ]

    private func nativeMenuViewport(_ collection: XCUIElement) -> CGRect {
        let frame = collection.frame
        // UIKit exposes the collection's content-sized frame (874pt in the
        // captured failure), while its actual menu clips to a 520pt ancestor.
        let wrappers = app.otherElements.containing(.collectionView, identifier: nil).allElementsBoundByIndex
            .map { $0.frame }.filter {
                abs($0.minX - frame.minX) < 2 && abs($0.width - frame.width) < 2
                    && $0.height > 100 && $0.height <= frame.height
            }
        return (wrappers.min(by: { $0.height < $1.height }) ?? frame).intersection(app.frame)
    }

    private func revealNativeMenuAction(_ id: String) {
        let collection = app.collectionViews.firstMatch
        guard collection.exists || collection.waitForExistence(timeout: 2) else { return }
        var reversed = false
        var down = true
        for _ in 0..<14 {
            if menuControl(id).exists { return }
            guard collection.exists && collection.isHittable else { return }
            let viewport = nativeMenuViewport(collection).insetBy(dx: 16, dy: 16)
            guard !viewport.isNull && viewport.height > 100 else { return }
            let visibleBefore = collection.buttons.allElementsBoundByIndex.filter { $0.isHittable }.map { $0.label }
            let origin = collection.coordinate(withNormalizedOffset: .zero)
            let frame = collection.frame
            let startY = down ? viewport.maxY - 20 : viewport.minY + 20
            let endY = startY + (down ? -1 : 1) * min(220, viewport.height * 0.5)
            origin.withOffset(CGVector(dx: viewport.midX - frame.minX, dy: startY - frame.minY))
                .press(forDuration: 0.05, thenDragTo: origin.withOffset(CGVector(dx: viewport.midX - frame.minX, dy: endY - frame.minY)),
                       withVelocity: .slow, thenHoldForDuration: 0.1)
            if menuControl(id).exists { return }
            let visibleAfter = collection.buttons.allElementsBoundByIndex.filter { $0.isHittable }.map { $0.label }
            if visibleBefore == visibleAfter {
                if reversed { return }
                down.toggle(); reversed = true
            }
        }
    }

    private func tap(_ id: String) {
        dismissSystemSavePassword()
        if element("theme.modern").exists && ["chat.action.kick", "chat.action.mute", "chat.action.unmute"].contains(id)
            && !menuControl(id).exists && menuControl("chat.menu.moderation").exists {
            tap("chat.menu.moderation")
        }
        let control = wait(id)
        if !control.isEnabled {
            let enabled = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == true"), object: control)
            XCTAssertEqual(XCTWaiter.wait(for: [enabled], timeout: 15), .completed, "Control remained disabled: \(id)")
        }
        reveal(control)
        waitUntilHittable(control, description: id)
        if id == "chat.send" && element("theme.classic-v3").exists {
            // XCTest's implicit scroll-to-visible can return {-1,-1} for
            // this fixed React Native footer while the keyboard is present.
            // Use the verified visible element's center, never a fixed point.
            let frame = control.frame
            XCTAssertTrue(app.frame.contains(frame), "Composer Send is outside the screen")
            if app.keyboards.firstMatch.exists {
                XCTAssertLessThanOrEqual(frame.maxY, app.keyboards.firstMatch.frame.minY,
                    "Composer Send is covered by the keyboard")
            }
            control.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5)).tap()
        } else {
            control.tap()
        }
    }

    private func waitUntilHittable(_ control: XCUIElement, description: String) {
        // A reveal can dismiss the software keyboard and relayout a long form.
        // Keep time for stable samples after XCTest's synchronous drag completes.
        let deadline = Date().addingTimeInterval(15)
        var previousFrame: CGRect?
        var readySince: Date?
        var revealAttempts = 0
        while Date() < deadline {
            dismissSystemSavePassword()
            if control.exists {
                let frame = control.frame
                if control.isHittable {
                    let unchanged = previousFrame.map {
                        abs($0.minX - frame.minX) < 0.5 && abs($0.minY - frame.minY) < 0.5
                            && abs($0.width - frame.width) < 0.5 && abs($0.height - frame.height) < 0.5
                    } ?? false
                    if unchanged, let since = readySince, Date().timeIntervalSince(since) >= 0.15 { return }
                    if !unchanged { readySince = Date() }
                    previousFrame = frame
                } else {
                    // Keyboard/modal geometry can change between snapshots.
                    // Re-evaluate the actual viewport; one scroll step may be
                    // needed even if an earlier sample was already hittable.
                    readySince = nil; previousFrame = nil
                    if revealAttempts < 3 {
                        reveal(control, maxScrolls: 1)
                        revealAttempts += 1
                    }
                }
            } else {
                readySince = nil; previousFrame = nil
            }
            RunLoop.current.run(until: Date().addingTimeInterval(0.05))
        }
        // The caller taps exactly once only after consecutive stable samples.
        // Do not add a contradictory third snapshot or retry a failed tap.
        XCTFail("Control did not become stably touchable: \(description)")
    }

    private func dismissSystemSavePassword() {
        // iOS presents this as an in-app Sheet, not an interruption Alert.
        // Decline only the observed system prompt; preserve every app dialog.
        let sheet = app.sheets["Save Password?"].firstMatch
        guard sheet.exists, sheet.staticTexts["Save Password?"].exists,
              sheet.buttons["Not Now"].isHittable else { return }
        sheet.buttons["Not Now"].tap()
        let dismissed = XCTNSPredicateExpectation(predicate: NSPredicate(format: "exists == false"), object: sheet)
        XCTAssertEqual(XCTWaiter.wait(for: [dismissed], timeout: 5), .completed, "System password prompt remained visible")
    }

    /// Long legacy menus expose offscreen accessibility frames. Use the
    /// target's actual scroll viewport and correct direction, with short drags
    /// that cannot fling a just-revealed row past the opposite edge.
    private func reveal(_ control: XCUIElement, maxScrolls: Int = 14) {
        for _ in 0..<maxScrolls {
            if control.isHittable { return }
            let menu = element("menu.items")
            let candidates = (app.scrollViews.allElementsBoundByIndex + app.tables.allElementsBoundByIndex
                + app.collectionViews.allElementsBoundByIndex).filter { $0.isHittable }
            let containing = candidates.filter { scroll in
                !control.identifier.isEmpty && scroll.descendants(matching: .any).matching(identifier: control.identifier).count > 0
            }
            guard let scroll = menu.exists && menu.isHittable ? menu
                : (containing.min(by: { $0.frame.height < $1.frame.height })
                    ?? candidates.max(by: { $0.frame.height < $1.frame.height })) else { return }
            let frame = scroll.frame
            var viewport = (scroll.elementType == .collectionView ? nativeMenuViewport(scroll) : frame.intersection(app.frame))
                .insetBy(dx: 16, dy: 16)
            if app.keyboards.firstMatch.exists {
                let keyboardTop = app.keyboards.firstMatch.frame.minY
                if keyboardTop > viewport.minY && keyboardTop < viewport.maxY {
                    viewport.size.height = keyboardTop - viewport.minY - 12
                }
            }
            guard !viewport.isNull, viewport.height > 70 else { return }
            let target = control.frame
            let above = target.minY < viewport.minY
            let below = target.maxY > viewport.maxY
            // A fully visible but covered control indicates an unexpected
            // overlay; scrolling the background would conceal that failure.
            guard above || below else { return }
            let distance = above ? viewport.minY - target.minY : target.maxY - viewport.maxY
            let step = min(viewport.height * 0.5, max(48, distance + 24))
            let startY = above ? viewport.minY + 12 : viewport.maxY - 12
            let endY = startY + (above ? step : -step)
            let origin = scroll.coordinate(withNormalizedOffset: .zero)
            let start = origin.withOffset(CGVector(dx: viewport.midX - frame.minX, dy: startY - frame.minY))
            let end = origin.withOffset(CGVector(dx: viewport.midX - frame.minX, dy: endY - frame.minY))
            start.press(forDuration: 0.05, thenDragTo: end, withVelocity: .slow, thenHoldForDuration: 0.1)
        }
    }

    private func fill(_ id: String, _ value: String, secure: Bool = false) {
        let field = secure ? app.secureTextFields[id] : app.textFields[id]
        XCTAssertTrue(field.waitForExistence(timeout: 10), "Missing text field: \(id)")
        reveal(field)
        field.tap()
        if let current = field.value as? String, current != field.placeholderValue {
            field.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count))
        }
        field.typeText(value)
    }

    private func capture(_ name: String) {
        let screenshot = XCTAttachment(screenshot: app.screenshot())
        screenshot.name = name
        screenshot.lifetime = .keepAlways
        add(screenshot)
    }

    private func captureTree(_ name: String) {
        let tree = XCTAttachment(string: app.debugDescription)
        tree.name = name
        tree.lifetime = .keepAlways
        add(tree)
    }

    private func loginScreen() {
        if element("welcome.login").waitForExistence(timeout: 3) { tap("welcome.login") }
        _ = wait("login.username", timeout: 45)
        let ready = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == true"), object: wait("login.submit"))
        XCTAssertEqual(XCTWaiter.wait(for: [ready], timeout: 10), .completed, "Saved-login loading did not finish")
        XCTAssertTrue(element("login.username").isHittable, "An unexpected dialog covers Login")
    }

    private func configure(_ mode: String, port: String) {
        tap("login.connection")
        tap("connection.manual")
        selectTransport(mode)
        fill("connection.host", "127.0.0.1")
        fill("connection.port", port)
        // Tap the action directly; its handler dismisses the native keyboard.
        tap("connection.save")
        loginScreen()
    }

    private func selectTransport(_ mode: String) {
        let identified = element("connection.\(mode)")
        if identified.exists { tap("connection.\(mode)"); return }
        // SwiftUI may project an option Text's identifier onto its segmented
        // button. If it does not, use only the exact label in that control.
        let segmented = app.segmentedControls.containing(.button, identifier: mode.uppercased()).firstMatch
        XCTAssertTrue(segmented.exists || segmented.waitForExistence(timeout: 5), "Missing native transport picker")
        let option = segmented.buttons.matching(NSPredicate(format: "label == %@", mode.uppercased())).firstMatch
        XCTAssertTrue(option.exists, "Missing exact native transport option: \(mode)")
        waitUntilHittable(option, description: "Transport \(mode)")
        option.tap()
        XCTAssertTrue(option.isSelected, "Native transport picker did not select \(mode)")
    }

    private func toggleLoginPreference(_ id: String) {
        guard element("theme.modern").exists else { tap(id); return }
        if ["login.auto", "login.invisible"].contains(id) {
            let control = wait(id)
            let previous = menuPreferenceState(control)
            tap(id)
            _ = wait(id) // Native menus may dismiss after changing a Toggle.
            let changed = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                self.menuPreferenceState(self.menuControl(id)) != previous
            }, object: nil)
            XCTAssertEqual(XCTWaiter.wait(for: [changed], timeout: 5), .completed, "Native menu preference did not toggle: \(id)")
            // Close through a real nonmutating destination, then return.
            tap("appearance.open"); tap("appearance.back")
            return
        }
        let control = app.switches[id].firstMatch
        XCTAssertTrue(control.exists || control.waitForExistence(timeout: 5), "Modern preference is not a native switch: \(id)")
        let previous = control.value as? String
        XCTAssertTrue(previous == "0" || previous == "1", "Native preference has no authoritative switch state: \(id)")
        // SwiftUI exposes a full-width labelled Switch containing the actual
        // UISwitch. Tap its control, not the noninteractive label's center.
        let toggle = control.switches.firstMatch.exists ? control.switches.firstMatch : control
        reveal(toggle)
        waitUntilHittable(toggle, description: id)
        toggle.tap()
        let changed = XCTNSPredicateExpectation(predicate: NSPredicate(format: "value == %@", previous == "1" ? "0" : "1"), object: control)
        XCTAssertEqual(XCTWaiter.wait(for: [changed], timeout: 5), .completed, "Native preference did not toggle: \(id)")
    }

    private func menuPreferenceState(_ control: XCUIElement) -> Bool {
        // UIKit may project a SwiftUI Menu Toggle as a selected action or a
        // switch value. Both are native state, never a glyph in the title.
        let value = (control.value as? String ?? "").lowercased()
        return control.isSelected || value == "1" || value == "on"
    }

    private func chooseTheme(_ theme: String, fromMenu: Bool = false) {
        if fromMenu { tap("command.menu") }
        tap("appearance.open")
        tap("appearance.\(theme)")
        _ = wait("theme.\(theme)")
        capture("appearance-\(theme)")
        tap("appearance.back")
    }

    private func editor() -> XCUIElement {
        let input = app.textViews["chat.message"]
        XCTAssertTrue(input.waitForExistence(timeout: 10), "Missing message editor")
        return input
    }

    private func assertReply(_ body: String, timeout: TimeInterval = 25) {
        let reply = app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", "Received: " + body)).firstMatch
        XCTAssertTrue(reply.waitForExistence(timeout: timeout), "Missing Bob's round-trip reply: \(body)")
    }

    private func assertEmptyComposerSendDisabled() {
        let input = editor()
        let empty = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let value = input.value as? String ?? ""
            return value.isEmpty || value == input.placeholderValue
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [empty], timeout: 10), .completed, "Acknowledged message did not clear its composer")
        let disabled = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == false"), object: wait("chat.send"))
        XCTAssertEqual(XCTWaiter.wait(for: [disabled], timeout: 5), .completed, "Empty composer retained an enabled Send control")
    }

    private func assertRoomText(_ text: String, timeout: TimeInterval = 25) {
        let row = app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", text)).firstMatch
        XCTAssertTrue(row.waitForExistence(timeout: timeout), "Missing room transcript text: \(text)")
    }

    private func exerciseLongTranscript() throws {
        let markers = (1...8).map { "Pinned history \($0):" }
        for marker in markers {
            _ = try peer(["action": "private-send", "body": marker + " Original-protocol messages build real history. Every line should remain readable above the floating composer."])
        }
        let latestMarker = "Latest reply stays above composer"
        _ = try peer(["action": "private-send", "body": latestMarker])
        func row(_ marker: String) -> XCUIElement {
            app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", marker)).firstMatch
        }
        func visibleTranscript() -> CGRect {
            let transcript = element("chat.screen").frame.intersection(app.frame)
            let top = max(transcript.minY, element("command.menu").frame.maxY, element("tabs.items").frame.maxY) + 12
            return CGRect(x: transcript.minX + 16, y: top, width: transcript.width - 32,
                          height: max(0, editor().frame.minY - 8 - top))
        }
        func fullyVisible(_ frame: CGRect) -> Bool {
            let viewport = visibleTranscript()
            return frame.width > 0 && frame.height > 0 && frame.minY >= viewport.minY && frame.maxY <= viewport.maxY
        }
        func stableFrame(_ target: XCUIElement, within condition: @escaping (CGRect) -> Bool) -> Bool {
            var previous: CGRect?
            var stableSince = Date()
            let settled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                guard target.exists else { previous = nil; return false }
                let frame = target.frame
                guard condition(frame) else { previous = nil; return false }
                if let old = previous, abs(old.minX - frame.minX) < 0.5, abs(old.minY - frame.minY) < 0.5,
                   abs(old.width - frame.width) < 0.5, abs(old.height - frame.height) < 0.5 {
                    return Date().timeIntervalSince(stableSince) >= 0.15
                }
                previous = frame
                stableSince = Date()
                return false
            }, object: nil)
            return XCTWaiter.wait(for: [settled], timeout: 15) == .completed
        }
        let latest = row(latestMarker)
        XCTAssertTrue(stableFrame(latest, within: fullyVisible), "Growing transcript left the newest message clipped or beneath the composer")
        capture("modern-long-transcript")

        // A real drag changes the reading position. Never scroll to repair the
        // preceding pinned assertion, or to force the next incoming row visible.
        var anchor: XCUIElement?
        for _ in 0..<5 {
            let viewport = visibleTranscript()
            XCTAssertGreaterThan(viewport.height, 160, "Insufficient visible transcript for a bounded history drag")
            let origin = app.coordinate(withNormalizedOffset: .zero)
            origin.withOffset(CGVector(dx: viewport.midX, dy: viewport.minY + viewport.height * 0.25))
                .press(forDuration: 0.05,
                       thenDragTo: origin.withOffset(CGVector(dx: viewport.midX, dy: viewport.minY + viewport.height * 0.75)),
                       withVelocity: .slow, thenHoldForDuration: 0.2)
            if !latest.exists || !fullyVisible(latest.frame) {
                anchor = markers.reversed().map(row).first { $0.exists && fullyVisible($0.frame) }
                if anchor != nil { break }
            }
        }
        let history = try XCTUnwrap(anchor, "The bounded gesture did not reveal an earlier complete message")
        XCTAssertTrue(stableFrame(history, within: fullyVisible), "History did not settle after the reading gesture")
        let readingFrame = history.frame
        let arrival = "Incoming while reading history"
        _ = try peer(["action": "private-send", "body": arrival])
        // The short history remains mounted in FlatList's render window. An
        // offscreen AX row proves delivery without changing the reading position.
        XCTAssertTrue(row(arrival).waitForExistence(timeout: 20), "Incoming history-check message was not delivered")
        XCTAssertTrue(stableFrame(history) { frame in
            fullyVisible(frame) && abs(frame.minY - readingFrame.minY) <= 2
        }, "An incoming message pulled the user away from the history they were reading")
        capture("modern-history-position-retained")
    }

    private func openRoomCatalog(category: String = "1") {
        if element("rooms.open").exists { tap("rooms.open") }
        else if element("tab.rooms").exists { tap("tab.rooms") }
        else {
            tap("command.menu")
            // Recovered v3 chat menus keep extra destinations in More Options.
            if !element("menu.rooms").exists && element("chat.more").exists { tap("chat.more") }
            tap("menu.rooms")
        }
        _ = wait("rooms.screen")
        setExpanded("rooms.category.\(category)", true)
    }

    private func setExpanded(_ id: String, _ expanded: Bool) {
        let control = wait(id)
        let expected = expanded ? "expanded" : "collapsed"
        let current = (control.value as? String ?? "").lowercased()
        XCTAssertTrue(current.contains("expanded") || current.contains("collapsed"), "Missing disclosure state: \(id)")
        if !current.contains(expected) { tap(id) }
        assertExpanded(id, expanded)
    }

    private func assertExpanded(_ id: String, _ expanded: Bool) {
        let expected = expanded ? "expanded" : "collapsed"
        let state = XCTNSPredicateExpectation(predicate: NSPredicate(format: "value CONTAINS[cd] %@", expected), object: wait(id))
        XCTAssertEqual(XCTWaiter.wait(for: [state], timeout: 5), .completed, "Disclosure did not remain \(expected): \(id)")
    }

    private func selectRoom(_ name: String) {
        let row = wait("rooms.room.\(name)")
        let ready = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == true"), object: row)
        XCTAssertEqual(XCTWaiter.wait(for: [ready], timeout: 15), .completed, "Room action did not finish")
        tap("rooms.room.\(name)")
        _ = wait("chat.screen")
    }

    private func writeRoomDraft(_ draft: String, theme: String) {
        if theme != "modern" { tap("chat.write") }
        let input = editor()
        let value = input.value as? String ?? ""
        XCTAssertTrue(value.isEmpty || value == input.placeholderValue, "Another room's draft leaked into this composer: \(value)")
        input.tap()
        input.typeText(draft)
        if theme != "modern" { tap("chat.back") }
    }

    private func assertRoomDraft(_ draft: String, theme: String, captureName: String? = nil) {
        if theme != "modern" { tap("chat.write") }
        XCTAssertEqual(editor().value as? String, draft, "Room selection or appearance switching lost its draft")
        if let captureName { capture(captureName) }
        if theme != "modern" { tap("chat.back") }
    }

    private func leaveCurrentRoom(_ name: String) {
        tap("command.menu")
        tap("room.leave")
        capture("room-\(name)-leave-confirmation")
        tap("room.leave.confirm")
        _ = wait("rooms.screen")
    }

    private func exerciseRoomFavorites(_ mode: String) {
        tap("command.menu")
        XCTAssertEqual(wait("room.favorite").label, "Add to Favorites")
        tap("room.favorite")
        openRoomCatalog(category: "2")
        _ = wait("rooms.room.Lobby")
        capture("\(mode)-room-favorite-added")
        selectRoom("Lobby")
        tap("command.menu")
        XCTAssertEqual(wait("room.favorite").label, "Remove from Favorites")
        tap("room.favorite")
        openRoomCatalog(category: "2")
        let empty = app.staticTexts["No favorite rooms yet."]
        XCTAssertTrue(empty.waitForExistence(timeout: 20), "Removed favorite remained in the authoritative catalog")
        XCTAssertFalse(element("rooms.room.Lobby").exists)
        capture("\(mode)-room-favorite-removed")
        setExpanded("rooms.category.1", true)
        selectRoom("Lobby")
    }

    private func exerciseRooms(_ mode: String, theme: String) throws {
        chooseTheme(theme)
        let port = try XCTUnwrap(environment["MIG33_SMOKE_\(mode.uppercased())_PORT"])
        configure(mode, port: port)
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        if theme == "modern" { _ = wait("glass.native") }

        openRoomCatalog()
        _ = wait("rooms.room.Lobby")
        _ = wait("rooms.room.Gaming")
        capture("\(mode)-\(theme)-room-catalog")
        fill("rooms.filter", "lob")
        _ = wait("rooms.room.Lobby")
        XCTAssertFalse(element("rooms.room.Gaming").exists, "Local room filter did not narrow the loaded category")
        capture("\(mode)-\(theme)-room-filtered")
        selectRoom("Lobby")
        // Go returns701, ACK1 and welcome500 in the same join response. The
        // transcript must already exist when that first room message arrives.
        assertRoomText("Welcome to Lobby.")
        capture("\(mode)-\(theme)-room-welcome")
        tap("command.menu")
        tap("room.participants")
        _ = wait("participants.screen")
        _ = wait("participants.user.bob")
        _ = wait("participants.user.alice")
        capture("\(mode)-\(theme)-room-participants")
        tap("participants.back")
        _ = wait("chat.screen")

        // The participant action opens an ordinary private conversation, while
        // the joined room remains available in the same navigation strip.
        tap("command.menu")
        tap("room.participants")
        tap("participants.user.bob")
        _ = wait("chat.screen")
        if theme != "modern" { tap("chat.write") }
        let privateMessage = "Hello from iOS \(mode.uppercased())"
        editor().tap()
        editor().typeText(privateMessage)
        tap("chat.send")
        _ = wait("chat.screen")
        assertReply(privateMessage)
        capture("\(mode)-\(theme)-room-participant-private-reply")
        tap("tab.room:Lobby")
        _ = wait("chat.screen")
        assertRoomText("Welcome to Lobby.", timeout: 5)

        exerciseRoomFavorites(mode)
        if theme != "modern" { tap("chat.write") }
        let message = "Hello room from iOS \(mode.uppercased())"
        editor().tap()
        editor().typeText(message)
        capture("\(mode)-\(theme)-room-compose")
        tap("chat.send")
        _ = wait("chat.screen")
        assertRoomText("Room reply: " + message)
        capture("\(mode)-\(theme)-room-reply")
        captureTree("\(mode)-\(theme)-room-reply-tree")

        let lobbyDraft = "Unsent Lobby draft \(mode)"
        let gamingDraft = "Different Gaming draft \(mode)"
        writeRoomDraft(lobbyDraft, theme: theme)
        openRoomCatalog()
        selectRoom("Gaming")
        assertRoomText("Welcome to Gaming.")
        writeRoomDraft(gamingDraft, theme: theme)
        openRoomCatalog()
        selectRoom("Lobby")
        assertRoomDraft(lobbyDraft, theme: theme)
        let themes = theme == "modern" ? ["classic", "classic-v3", "modern"] : ["modern", "classic", "classic-v3"]
        for nextTheme in themes {
            chooseTheme(nextTheme, fromMenu: true)
            _ = wait("chat.screen")
            assertRoomText("Room reply: " + message, timeout: 5)
            assertRoomDraft(lobbyDraft, theme: nextTheme, captureName: "\(mode)-room-\(nextTheme)-draft-retained")
        }

        openRoomCatalog()
        selectRoom("Gaming")
        assertRoomDraft(gamingDraft, theme: theme)
        leaveCurrentRoom("Gaming")
        setExpanded("rooms.category.1", true)
        selectRoom("Lobby")
        assertRoomDraft(lobbyDraft, theme: theme)
        leaveCurrentRoom("Lobby")
        capture("\(mode)-\(theme)-room-left-catalog")
        if theme == "classic-v3" { tap("rooms.back") }
        XCTAssertFalse(element("tab.room:Lobby").exists, "Leaving kept the Lobby conversation open")
        XCTAssertFalse(element("tab.room:Gaming").exists, "Leaving kept the Gaming conversation open")
        logout("\(mode)-rooms")
    }

    /// Commands act only through real fixture gateway clients. The listener is
    /// loopback-only, authenticated by a per-run token and absent in the app.
    private func peer(_ input: [String: Any]) throws -> [String: Any] {
        let url = try XCTUnwrap(URL(string: XCTUnwrap(environment["MIG33_SMOKE_CONTROL_URL"])))
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 20
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(try XCTUnwrap(environment["MIG33_SMOKE_CONTROL_TOKEN"]), forHTTPHeaderField: "X-Smoke-Token")
        request.httpBody = try JSONSerialization.data(withJSONObject: input)
        let completed = expectation(description: "Original gateway fixture command")
        var data: Data?
        var response: URLResponse?
        var failure: Error?
        URLSession.shared.dataTask(with: request) { body, reply, error in
            data = body; response = reply; failure = error; completed.fulfill()
        }.resume()
        wait(for: [completed], timeout: 25)
        if let failure { throw failure }
        let body = try XCTUnwrap(data)
        let result = try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200, "Fixture rejected command: \(result)")
        return result
    }

    private func pauseForDelivery(_ seconds: TimeInterval = 0.15) {
        let elapsed = expectation(description: "Gateway delivery interval")
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds) { elapsed.fulfill() }
        wait(for: [elapsed], timeout: seconds + 2)
    }

    private func assertMember(_ name: String, in room: String, present: Bool, muted: Bool? = nil, actor: String = "carol") throws {
        for _ in 0..<40 {
            let response = try peer(["action": "room", "actor": actor, "room": room])
            let people = response["participants"] as? [[String: Any]] ?? []
            let member = people.first { ($0["username"] as? String) == name }
            if (member != nil) == present && (muted == nil || (member?["muted"] as? Bool) == muted) { return }
            pauseForDelivery()
        }
        XCTFail("Authoritative707 roster did not show \(name) present=\(present), muted=\(String(describing: muted)) in \(room)")
    }

    private func pageButton(_ label: String) {
        let control = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH %@ AND label == %@", "page.submit.", label)).firstMatch
        XCTAssertTrue(control.exists || control.waitForExistence(timeout: 15), "Missing original form submit: \(label)")
        if !control.isEnabled {
            let enabled = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == true"), object: control)
            XCTAssertEqual(XCTWaiter.wait(for: [enabled], timeout: 15), .completed, "Original form remained busy: \(label)")
        }
        reveal(control)
        waitUntilHittable(control, description: "Original form submit: \(label)")
        control.tap()
    }

    private func pageLink(_ label: String) {
        let control = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@ AND label == %@", "page.link.", label)).firstMatch
        XCTAssertTrue(control.exists || control.waitForExistence(timeout: 15), "Missing original page link: \(label)")
        if !control.isEnabled {
            let enabled = XCTNSPredicateExpectation(predicate: NSPredicate(format: "enabled == true"), object: control)
            XCTAssertEqual(XCTWaiter.wait(for: [enabled], timeout: 15), .completed, "Original link remained busy: \(label)")
        }
        reveal(control)
        waitUntilHittable(control, description: "Original page link: \(label)")
        control.tap()
    }

    private func fillMultiline(_ id: String, _ value: String) {
        let input = app.textViews[id]
        XCTAssertTrue(input.waitForExistence(timeout: 15), "Missing multiline field: \(id)")
        reveal(input)
        input.tap()
        if let existing = input.value as? String, existing != input.placeholderValue {
            input.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: existing.count))
        }
        input.typeText(value)
    }

    private func dismissActionNotice() {
        if element("notice.dismiss").waitForExistence(timeout: 1) { tap("notice.dismiss") }
        if element("participant.back").exists { tap("participant.back") }
        _ = wait("chat.screen")
    }

    private func closeOriginalPage() {
        for _ in 0..<10 {
            if !element("page.screen").exists { break }
            tap("page.back")
        }
        XCTAssertFalse(element("page.screen").exists, "Original-page Back did not return to the conversation")
        _ = wait("chat.screen")
    }

    private func selectModeration(_ action: String, user: String) {
        tap("command.menu")
        // The recovered Mute picker selects710 when708 marks the target muted.
        tap("chat.action.\(action == "unmute" ? "mute" : action)")
        tap("participant.select.\(user)")
        capture("feature-\(action)-confirmation")
        XCTAssertEqual(wait("action.confirm").label, action.capitalized)
        tap("action.confirm")
    }

    private func sendFeatureText(_ body: String, theme: String) {
        if theme != "modern" { tap("chat.write") }
        editor().tap(); editor().typeText(body)
        tap("chat.send")
        if theme == "modern" { assertEmptyComposerSendDisabled() }
        _ = wait("chat.screen")
    }

    private func inviteFeatureUser(_ username: String, theme: String) {
        tap("command.menu"); tap("chat.action.invite")
        _ = wait("invite.screen")
        if theme == "modern" { fill("invite.username", username); tap("invite.submit") }
        else { tap("invite.contact.\(username)") }
        _ = wait("chat.screen")
        if element("notice.dismiss").waitForExistence(timeout: 1) { tap("notice.dismiss") }
    }

    private func exerciseComposer(theme: String) throws {
        if theme != "modern" { tap("chat.write") }
        tap("chat.tools"); tap("composer.open.colors")
        _ = wait("composer.colors")
        tap("composer.color.default")
        tap("chat.tools"); tap("composer.open.emoticons")
        _ = wait("composer.emoticons")
        let choice = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "composer.emoticon.")).firstMatch
        XCTAssertTrue(choice.waitForExistence(timeout: 15), "Original916 supplied no usable emoticon")
        XCTAssertTrue(choice.isHittable)
        choice.tap()
        let shortcut = (editor().value as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        XCTAssertFalse(shortcut.isEmpty, "Selecting an original emoticon did not insert its shortcut")
        let receivedBefore = (try peer(["action": "private-received"])["bodies"] as? [String] ?? []).count
        // Programmatic insertion can retain a caret at the beginning. Send the
        // actual shortcut through500, then verify its acknowledged draft clear.
        tap("chat.send")
        _ = wait("chat.screen")
        if theme != "modern" { tap("chat.write") }
        let empty = XCTNSPredicateExpectation(predicate: NSPredicate(format: "value == '' OR value == 'Message'"), object: editor())
        XCTAssertEqual(XCTWaiter.wait(for: [empty], timeout: 15), .completed, "Emoticon send did not clear the draft")
        if theme != "modern" { tap("chat.back") }
        var delivered = false
        for _ in 0..<40 {
            let received = try peer(["action": "private-received"])["bodies"] as? [String] ?? []
            if received.dropFirst(receivedBefore).contains(shortcut) { delivered = true; break }
            pauseForDelivery()
        }
        XCTAssertTrue(delivered, "The original500 recipient did not receive the selected emoticon shortcut")
    }

    private func revealTab(_ id: String) {
        let tab = wait(id)
        if element("theme.modern").exists && ["tab.friends", "tab.rooms"].contains(id) {
            waitUntilHittable(tab, description: "Fixed navigation destination \(id)")
            return
        }
        let scroll = element("tabs.items")
        XCTAssertTrue(scroll.waitForExistence(timeout: 10))
        for _ in 0..<12 {
            let frame = scroll.frame
            // Native conversation padding is3pt. Adding a synthetic5pt inset
            // made a fully visible first tab fail this check forever.
            let viewport = frame.intersection(app.frame)
            let target = tab.frame
            if target.minX >= viewport.minX - 0.5 && target.maxX <= viewport.maxX + 0.5
                && target.minY >= viewport.minY - 0.5 && target.maxY <= viewport.maxY + 0.5 { return }
            let before = target.minX < viewport.minX
            let step = min(viewport.width * 0.6, max(40, before ? viewport.minX - target.minX : target.maxX - viewport.maxX))
            let startX = before ? viewport.minX + 12 : viewport.maxX - 12
            let origin = scroll.coordinate(withNormalizedOffset: .zero)
            origin.withOffset(CGVector(dx: startX - frame.minX, dy: frame.height / 2))
                .press(forDuration: 0.05,
                       thenDragTo: origin.withOffset(CGVector(dx: startX - frame.minX + (before ? step : -step), dy: frame.height / 2)),
                       withVelocity: .slow, thenHoldForDuration: 0.1)
        }
        let viewport = scroll.frame.intersection(app.frame)
        let target = tab.frame
        XCTAssertGreaterThanOrEqual(target.minX, viewport.minX - 0.5, "Unread tab's leading edge remains clipped")
        XCTAssertLessThanOrEqual(target.maxX, viewport.maxX + 0.5, "Unread tab's trailing edge remains clipped")
        XCTAssertGreaterThanOrEqual(target.minY, viewport.minY - 0.5, "Unread tab's top remains clipped")
        XCTAssertLessThanOrEqual(target.maxY, viewport.maxY + 0.5, "Unread tab's bottom remains clipped")
        XCTAssertTrue(tab.isHittable, "Unread tab could not be revealed")
    }

    /// Change the real iOS accessibility preference while logged out. Opening
    /// Settings intentionally backgrounds and suspends the app connection.
    private func setSystemReduceMotion(_ enabled: Bool, returnToApp: Bool = true) throws -> Bool {
        try setSystemAccessibility(.motion, enabled, returnToApp: returnToApp)
    }

    private func setSystemAccessibility(_ preference: SystemAccessibility, _ enabled: Bool, returnToApp: Bool = true) throws -> Bool {
        let settings = XCUIApplication(bundleIdentifier: "com.apple.Preferences")
        settings.launch()
        defer { if settings.state != .notRunning { settings.terminate() } }
        func failure(_ message: String) -> NSError {
            let screenshot = XCTAttachment(screenshot: settings.screenshot())
            screenshot.name = "\(preference.rawValue)-settings-failure"
            screenshot.lifetime = .keepAlways; add(screenshot)
            let tree = XCTAttachment(string: settings.debugDescription)
            tree.name = "\(preference.rawValue)-settings-failure-tree"
            tree.lifetime = .keepAlways; add(tree)
            return NSError(domain: "Mig33NativeSmoke", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
        }
        for _ in 0..<10 {
            if settings.navigationBars["Settings"].exists { break }
            let back = settings.navigationBars.buttons.firstMatch
            if !back.exists { break }
            back.tap()
        }
        func openRow(_ name: String) throws {
            let cell = settings.cells.containing(.staticText, identifier: name).firstMatch
            let row = cell.exists ? cell : settings.staticTexts[name].firstMatch
            for _ in 0..<12 { if row.exists && row.isHittable { break }; settings.swipeUp() }
            guard row.exists && row.isHittable else { throw failure("Missing iOS Settings row: \(name)") }
            row.tap()
        }
        try openRow("Accessibility")
        try openRow(preference.section)
        let toggle = settings.switches.matching(NSPredicate(format: "label == %@", preference.rawValue)).firstMatch
        for _ in 0..<8 { if toggle.exists && toggle.isHittable { break }; settings.swipeUp() }
        guard toggle.exists else { throw failure("Missing system \(preference.rawValue) switch") }
        let previous = (toggle.value as? String) == "1"
        if accessibilityOriginal[preference] == nil {
            accessibilityOriginal[preference] = previous
            // XCTest teardown runs even when continueAfterFailure interrupts
            // the test, unlike relying only on the test function's defer.
            addTeardownBlock { [weak self] in
                guard let self, self.accessibilityNeedsRestore.contains(preference), let original = self.accessibilityOriginal[preference] else { return }
                _ = try self.setSystemAccessibility(preference, original, returnToApp: false)
            }
        }
        if previous != enabled {
            accessibilityNeedsRestore.insert(preference)
            let cell = toggle.identifier.isEmpty ? settings.cells.containing(.staticText, identifier: preference.rawValue).firstMatch
                : settings.cells[toggle.identifier]
            let rowFrame = toggle.frame
            let inCell = cell.exists ? cell.switches.allElementsBoundByIndex : []
            // iOS exposes a labeled330pt row and an unlabeled63pt UISwitch.
            // Select the real control, not the row's center over its text.
            let sameRow = settings.switches.allElementsBoundByIndex.filter {
                abs($0.frame.midY - rowFrame.midY) < 3 && $0.frame.width > 0
            }
            guard let control = (inCell + sameRow).min(by: { $0.frame.width < $1.frame.width }) else {
                throw failure("Missing native \(preference.rawValue) toggle within its Settings row")
            }
            guard control.frame.width <= 100 || control.frame.width < rowFrame.width / 2 else {
                throw failure("Settings exposed only the \(preference.rawValue) label row, not its native toggle")
            }
            if !control.isHittable {
                let hittable = XCTNSPredicateExpectation(predicate: NSPredicate(format: "hittable == true"), object: control)
                guard XCTWaiter.wait(for: [hittable], timeout: 5) == .completed else {
                    throw failure("Native \(preference.rawValue) toggle was not touchable")
                }
            }
            control.tap()
        }
        let changed = XCTNSPredicateExpectation(predicate: NSPredicate(format: "value == %@", enabled ? "1" : "0"), object: toggle)
        guard XCTWaiter.wait(for: [changed], timeout: 5) == .completed else {
            throw failure("System \(preference.rawValue) value did not become \(enabled ? "1" : "0")")
        }
        if accessibilityOriginal[preference] == enabled { accessibilityNeedsRestore.remove(preference) }
        else { accessibilityNeedsRestore.insert(preference) }
        settings.terminate()
        if returnToApp { try relaunch(); loginScreen() }
        return previous
    }

    private func exerciseUnread(_ mode: String, steady: Bool = false) throws {
        chooseTheme("modern")
        configure(mode, port: try XCTUnwrap(environment["MIG33_SMOKE_\(mode.uppercased())_PORT"]))
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        tap("friends.contact.bob")
        _ = wait("chat.screen")
        tap("tab.friends")
        let body = "Unread \(mode.uppercased()) \(steady ? "steady" : "pulsing")"
        _ = try peer(["action": "private-send", "body": body])
        let tab = wait("tab.peer:bob")
        let unread = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label CONTAINS %@", "unread messages"), object: tab)
        XCTAssertEqual(XCTWaiter.wait(for: [unread], timeout: 15), .completed)
        for theme in ["modern", "classic", "classic-v3"] {
            if theme != "modern" { chooseTheme(theme, fromMenu: true) }
            revealTab("tab.peer:bob")
            XCTAssertTrue(tab.label.contains("unread messages"), "Theme switch cleared an unread conversation")
            let initialFrame = tab.frame
            for phase in steady ? ["steady"] : ["on", "off"] {
                let plate = wait("tab.unread-\(phase).peer:bob", timeout: 10)
                XCTAssertGreaterThan(plate.frame.width, 20)
                XCTAssertGreaterThan(plate.frame.height, 20)
                capture("\(mode)-\(theme)-unread-\(phase)")
                if theme == "modern" {
                    XCTAssertEqual(tab.frame.width, initialFrame.width, accuracy: 1, "Unread material changed the native tab width")
                    XCTAssertEqual(tab.frame.height, initialFrame.height, accuracy: 1, "Unread material changed the native tab height")
                }
            }
            if steady {
                let flashing = XCTNSPredicateExpectation(predicate: NSPredicate(format: "exists == true"), object: element("tab.unread-off.peer:bob"))
                flashing.isInverted = true
                XCTAssertEqual(XCTWaiter.wait(for: [flashing], timeout: 1.7), .completed, "Reduce Motion did not hold the complete unread plate steady")
            }
            if theme == "modern" {
                if mode == "https", !steady {
                    // Sample the actual native interpolation, not just the
                    // JS phase markers, for visual review of the amber field.
                    for sample in 0..<6 {
                        capture("modern-unread-motion-\(sample)")
                        Thread.sleep(forTimeInterval: 0.15)
                    }
                }
                // Press the real native control while its internal field is
                // animating, then rearm unread for the Classic retention checks.
                _ = wait("tab.unread-\(steady ? "steady" : "on").peer:bob", timeout: 10)
                tap("tab.peer:bob")
                _ = wait("chat.screen")
                assertRoomText(body)
                XCTAssertFalse(tab.label.contains("unread messages"), "Native glass tab did not open and clear unread")
                capture("\(mode)-modern-unread-touch-cleared")
                tap("tab.friends")
                _ = try peer(["action": "private-send", "body": body + " after native tap"])
                let rearmed = XCTNSPredicateExpectation(predicate: NSPredicate(format: "label CONTAINS %@", "unread messages"), object: tab)
                XCTAssertEqual(XCTWaiter.wait(for: [rearmed], timeout: 15), .completed)
            }
        }
        tap("tab.peer:bob")
        _ = wait("chat.screen")
        assertRoomText(body)
        XCTAssertFalse(tab.label.contains("unread messages"), "Opening Bob did not clear the unread label")
        for phase in ["on", "off", "steady"] { XCTAssertFalse(element("tab.unread-\(phase).peer:bob").exists) }
        capture("\(mode)-unread-selected-cleared")
        logout("\(mode)-unread")
    }

    private func expandNativeSheet(_ name: String) {
        let sheet = wait("sheet.native")
        if sheet.frame.height <= 100 {
            let laidOut = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in sheet.frame.height > 100 }, object: sheet)
            XCTAssertEqual(XCTWaiter.wait(for: [laidOut], timeout: 5), .completed)
        }
        XCTAssertGreaterThan(sheet.frame.height, 100, "The native sheet did not lay out its React Native content")
        let mediumHeight = sheet.frame.height
        capture("modern-\(name)-sheet-medium")
        // The native grabber has no custom identifier. Drag the header directly
        // above the measured content; do not scroll the embedded picker grid.
        let origin = app.coordinate(withNormalizedOffset: .zero)
        let start = origin.withOffset(CGVector(dx: sheet.frame.midX, dy: sheet.frame.minY - 20))
        let end = origin.withOffset(CGVector(dx: sheet.frame.midX, dy: app.frame.height * 0.16))
        start.press(forDuration: 0.05, thenDragTo: end, withVelocity: .slow, thenHoldForDuration: 0.1)
        let enlarged = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            sheet.frame.height > mediumHeight + 80
        }, object: sheet)
        XCTAssertEqual(XCTWaiter.wait(for: [enlarged], timeout: 5), .completed,
                       "The real native sheet did not expand from medium to large")
        capture("modern-\(name)-sheet-large")
    }

    private func exerciseModernPresentation() throws {
        chooseTheme("modern")
        configure("https", port: try XCTUnwrap(environment["MIG33_SMOKE_HTTPS_PORT"]))
        let logo = app.images["mig33"].firstMatch
        XCTAssertTrue(logo.exists || logo.waitForExistence(timeout: 10), "Missing original mig33 mark on Modern Login")
        XCTAssertTrue(app.switches["login.remember"].firstMatch.exists, "Remember Login is not a native switch")
        XCTAssertFalse(element("login.options").exists, "Redundant preference disclosure remains")
        for id in ["login.auto", "login.invisible", "login.connection", "appearance.open"] {
            XCTAssertFalse(element(id).exists, "Secondary option is visible outside the sign-in menu: \(id)")
        }
        let options = wait("login.menu")
        XCTAssertGreaterThanOrEqual(options.frame.width, 44)
        XCTAssertGreaterThanOrEqual(options.frame.height, 44)
        capture("modern-login-uncluttered")
        tap("login.menu")
        for id in ["login.auto", "login.invisible", "login.connection", "appearance.open"] { _ = wait(id) }
        capture("modern-sign-in-menu")
        captureTree("modern-sign-in-menu-state")
        toggleLoginPreference("login.auto")
        XCTAssertEqual(app.switches["login.remember"].firstMatch.value as? String, "1", "Auto Login did not enable Remember Login")
        toggleLoginPreference("login.auto")
        toggleLoginPreference("login.remember")
        for _ in 0..<2 { toggleLoginPreference("login.invisible") }
        capture("modern-authentic-login")
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        // Arrival is transient and must not hold authentication or interaction
        // open for a screenshot. Assert the stable destination; use a recording
        // for inspection of the brief material/brand transition itself.
        _ = wait("friends.screen", timeout: 35)
        _ = wait("glass.native")
        capture("modern-friends-arrived")
        captureTree("modern-friends-native-navigation")
        setExpanded("friends.group.1", false)
        XCTAssertFalse(element("friends.contact.bob").exists)
        tap("tab.rooms"); _ = wait("rooms.screen")
        tap("tab.friends"); _ = wait("friends.screen")
        assertExpanded("friends.group.1", false)
        XCTAssertFalse(element("friends.contact.bob").exists, "Navigation expanded a collapsed Friends group")
        capture("modern-friends-collapse-retained")
        setExpanded("friends.group.1", true)
        tap("friends.options.bob")
        _ = wait("chat.action.private")
        capture("modern-friends-native-options")
        tap("chat.action.private")
        _ = wait("chat.screen")
        editor().tap(); editor().typeText("Hello from iOS HTTPS")
        let keyboard = app.keyboards.firstMatch
        XCTAssertTrue(keyboard.exists, "Modern composer did not expose the native keyboard responder")
        let keyboardFrame = keyboard.frame.intersection(app.frame)
        let visibleKeys = keyboard.keys.allElementsBoundByIndex.filter { $0.isHittable }
        let softwareKeyboardVisible = !keyboardFrame.isNull && keyboardFrame.height > 160
            && keyboardFrame.width > app.frame.width * 0.8 && visibleKeys.count > 10
        if softwareKeyboardVisible {
            XCTAssertLessThanOrEqual(editor().frame.maxY, keyboardFrame.minY + 1, "Software keyboard covers the message input")
            XCTAssertLessThanOrEqual(wait("chat.send").frame.maxY, keyboardFrame.minY + 1, "Software keyboard covers Send")
        }
        capture(softwareKeyboardVisible ? "modern-private-software-keyboard" : "modern-private-editor")
        if !softwareKeyboardVisible {
            let limitation = XCTAttachment(string: "UNVERIFIED: software keyboard geometry/avoidance. The simulator exposed a keyboard responder but fewer than 11 visible keys or less than 160pt of on-screen keyboard. A connected hardware keyboard can produce this state; typing and caret alone do not verify the software keyboard.")
            limitation.name = "software-keyboard-verification-unavailable"
            limitation.lifetime = .keepAlways
            add(limitation)
            captureTree("modern-private-keyboard-geometry")
        }
        tap("chat.send")
        assertReply("Hello from iOS HTTPS")
        assertEmptyComposerSendDisabled()
        capture("modern-private-reply")

        tap("chat.tools"); tap("composer.open.emoticons")
        _ = wait("composer.emoticons")
        expandNativeSheet("emoticons")
        tap("chat.sheet.close")
        _ = wait("chat.screen")
        tap("chat.tools"); tap("composer.open.colors")
        _ = wait("sheet.native")
        tap("composer.color.default")
        _ = wait("chat.screen")

        tap("command.menu"); tap("chat.action.block")
        XCTAssertTrue(app.alerts.firstMatch.waitForExistence(timeout: 10), "Modern confirmation is not a native system alert")
        _ = wait("action.confirm")
        capture("modern-native-block-confirmation")
        let cancel = app.alerts.buttons["Cancel"].firstMatch
        waitUntilHittable(cancel, description: "Native confirmation Cancel")
        cancel.tap()
        _ = wait("chat.screen")

        tap("tab.friends")
        _ = try peer(["action": "private-send", "body": "Modern unread HTTPS"])
        revealTab("tab.peer:bob")
        for phase in ["on", "off"] {
            _ = wait("tab.unread-\(phase).peer:bob", timeout: 10)
            capture("modern-polished-unread-\(phase)")
        }
        tap("tab.peer:bob")
        assertRoomText("Modern unread HTTPS")
        XCTAssertFalse(element("tab.peer:bob").label.contains("unread messages"))
        try exerciseLongTranscript()

        openRoomCatalog()
        _ = wait("rooms.room.Lobby")
        capture("modern-room-catalog")
        fill("rooms.filter", "lob")
        XCTAssertFalse(element("rooms.room.Gaming").exists, "Loaded-room filter did not narrow the catalog")
        tap("tab.friends"); _ = wait("friends.screen")
        tap("tab.rooms"); _ = wait("rooms.screen")
        XCTAssertEqual(app.textFields["rooms.filter"].value as? String, "lob", "Navigation lost the room filter")
        assertExpanded("rooms.category.1", true)
        _ = wait("rooms.room.Lobby")
        XCTAssertFalse(element("rooms.room.Gaming").exists, "Returning to Rooms lost the active filter")
        capture("modern-room-filter-retained")
        tap("rooms.more.Lobby")
        _ = wait("room.join")
        capture("modern-room-native-options")
        tap("room.join")
        _ = wait("chat.screen")
        sendFeatureText("Hello room from iOS HTTPS", theme: "modern")
        assertRoomText("Room reply: Hello room from iOS HTTPS")
        capture("modern-room-reply")
        tap("command.menu"); tap("room.participants")
        _ = wait("participants.user.bob")
        capture("modern-room-participants")
        tap("participants.back")
        tap("command.menu"); tap("room.leave")
        XCTAssertTrue(app.alerts.firstMatch.waitForExistence(timeout: 10), "Room leave did not present a native alert")
        capture("modern-native-room-leave")
        tap("room.leave.confirm")
        _ = wait("rooms.screen")
        logout("modern-presentation")
    }

    private func assertCommandDraft(_ draft: String) {
        let input = editor()
        let expected = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let value = input.value as? String ?? ""
            return value == draft || draft.isEmpty && value == input.placeholderValue
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [expected], timeout: 5), .completed, "Command selection changed or sent the draft unexpectedly")
    }

    private func assertCommandGeometry() {
        let panel = wait("composer.commands")
        let input = editor()
        var previous: CGRect?
        var stableSince = Date()
        let readable = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let frame = panel.frame
            let top = max(self.element("command.menu").frame.maxY, self.element("tabs.items").frame.maxY)
            guard frame.width > 0, frame.height >= 72, frame.minY >= top - 1,
                  frame.maxY <= input.frame.minY + 1, input.frame.maxY <= self.app.frame.maxY else {
                previous = nil; return false
            }
            if let old = previous, abs(old.minY - frame.minY) < 0.5, abs(old.height - frame.height) < 0.5 {
                return Date().timeIntervalSince(stableSince) >= 0.15
            }
            previous = frame; stableSince = Date(); return false
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [readable], timeout: 10), .completed,
                       "Command picker overlaps navigation, editor, or the visible screen")
    }

    private func exerciseCommands(_ mode: String, theme: String) throws {
        chooseTheme(theme)
        configure(mode, port: try XCTUnwrap(environment["MIG33_SMOKE_\(mode.uppercased())_PORT"]))
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        tap("friends.contact.bob")
        _ = wait("chat.screen")
        if theme != "modern" { tap("chat.write") }
        editor().tap(); editor().typeText("/")
        _ = wait("composer.commands")
        XCTAssertFalse(element("composer.command.games").exists, "Private chat offered a room-only command")
        XCTAssertFalse(element("composer.command.ban").exists, "Private chat offered a community-only command")
        if theme == "modern" { assertCommandGeometry() }
        capture("\(mode)-commands-list")
        tap("composer.commands.close")
        XCTAssertFalse(element("composer.commands").exists, "Dismissal kept the command picker open")
        assertCommandDraft("/")
        editor().typeText("wa")
        _ = wait("composer.command.wave")
        XCTAssertFalse(element("composer.command.hug").exists, "Command prefix did not filter suggestions")
        capture("\(mode)-commands-filtered")
        tap("composer.command.wave")
        assertCommandDraft("/wave ")
        XCTAssertTrue(app.keyboards.firstMatch.exists, "Command selection lost the native editor responder")
        // Typing into the retained editor proves focus/caret preservation;
        // keyboard existence alone does not prove an onscreen software layout.
        editor().typeText("x")
        assertCommandDraft("/wave x")
        editor().typeText(XCUIKeyboardKey.delete.rawValue)
        assertCommandDraft("/wave ")
        XCTAssertFalse(app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", "alice waves.")).firstMatch.exists,
                       "Selecting a command executed it before Send")
        capture("\(mode)-commands-completed-unsent")
        tap("chat.send")
        _ = wait("chat.screen")
        assertRoomText("alice waves.")
        if theme == "modern" { assertEmptyComposerSendDisabled() }
        capture("\(mode)-commands-explicit-send")

        if theme != "modern" { tap("chat.write") }
        editor().tap(); editor().typeText("/gift ")
        _ = wait("composer.command.gift-list")
        capture("\(mode)-commands-gift-subcommands")
        editor().typeText("se")
        tap("composer.command.gift-search")
        assertCommandDraft("/gift search ")
        XCTAssertFalse(element("notice.dismiss").exists, "Gift completion executed a server request")
        editor().typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: "/gift search ".count))
        assertCommandDraft("")
        if theme != "modern" { tap("chat.back") }

        openRoomCatalog()
        selectRoom("Lobby")
        assertRoomText("Welcome to Lobby")
        if theme != "modern" { tap("chat.write") }
        editor().tap(); editor().typeText("/")
        _ = wait("composer.commands")
        XCTAssertTrue(element("composer.command.games").exists, "Joined room omitted /games")
        XCTAssertFalse(element("composer.command.ban").exists, "Unlinked room offered a community-only command")
        if theme == "modern" { assertCommandGeometry() }
        editor().typeText("ga")
        tap("composer.command.games")
        assertCommandDraft("/games")
        capture("\(mode)-commands-room-context")
        // /games may open a server catalog. This flow checks insertion only,
        // then exercises a real room action through the ordinary Send path.
        editor().typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: "/games".count))
        editor().typeText("/wa")
        tap("composer.command.wave")
        assertCommandDraft("/wave ")
        tap("chat.send")
        _ = wait("chat.screen")
        assertRoomText("alice waves.")
        capture("\(mode)-commands-room-action")
        leaveCurrentRoom("Lobby")
        if theme == "classic-v3" { tap("rooms.back") }
        logout("\(mode)-commands", verifyPresentation: true)
    }

    private func exerciseModernAccessibility(_ name: String, opaque: Bool) throws {
        _ = wait("theme.modern")
        configure("https", port: try XCTUnwrap(environment["MIG33_SMOKE_HTTPS_PORT"]))
        capture("modern-\(name)-login-controls")
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        _ = wait(opaque ? "glass.solid" : "glass.native")
        if opaque { XCTAssertFalse(element("glass.native").exists, "Reduce Transparency retained a transparent app glass surface") }
        capture("modern-\(name)-friends")
        tap("friends.contact.bob")
        tap("chat.tools"); tap("composer.open.colors")
        _ = wait("sheet.native")
        _ = wait("composer.color.default")
        capture("modern-\(name)-native-sheet")
        tap("chat.sheet.close")
        logout("modern-\(name)")
    }

    private func exerciseFeatures(_ mode: String, theme: String) throws {
        chooseTheme(theme)
        configure(mode, port: try XCTUnwrap(environment["MIG33_SMOKE_\(mode.uppercased())_PORT"]))
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        if theme == "modern" { _ = wait("glass.native") }

        let suffix = mode.uppercased()
        let room = "Native " + suffix
        let description = "café & + = 50% — native " + suffix
        openRoomCatalog()
        tap("command.menu"); tap("rooms.create")
        _ = wait("page.screen")
        fill("page.field.name", room)
        pageButton("Continue")
        fillMultiline("page.field.description", description)
        capture("\(mode)-feature-room-description-unicode")
        pageButton("Create Room")
        assertRoomText("Your room is ready.")
        pageLink("Room Info")
        assertRoomText(description)
        capture("\(mode)-feature-room-info-roundtrip")
        pageLink("Join Room")
        _ = wait("chat.screen")
        assertRoomText("Welcome to " + room)
        _ = try peer(["action": "join", "room": room])

        // Query through the UI first so moderator gating uses a real708.
        tap("command.menu"); tap("room.participants")
        _ = wait("participants.user.bob")
        tap("participants.back")
        selectModeration("mute", user: "bob")
        try assertMember("bob", in: room, present: true, muted: true)
        dismissActionNotice()
        let muted = try peer(["action": "room-send", "actor": "bob", "room": room, "body": "Muted probe " + suffix])
        XCTAssertEqual(muted["accepted"] as? Bool, false, "Room mute did not reject Bob's real500 send")
        capture("\(mode)-feature-muted")
        selectModeration("unmute", user: "bob")
        try assertMember("bob", in: room, present: true, muted: false)
        dismissActionNotice()
        let unmuted = try peer(["action": "room-send", "actor": "bob", "room": room, "body": "Unmuted probe " + suffix])
        XCTAssertEqual(unmuted["accepted"] as? Bool, true)
        assertRoomText("Unmuted probe " + suffix)
        selectModeration("kick", user: "bob")
        try assertMember("bob", in: room, present: false)
        dismissActionNotice()
        capture("\(mode)-feature-moderator-kick")
        _ = try peer(["action": "join", "room": room, "actors": ["bob"]])
        leaveCurrentRoom(room)

        // Alice is an ordinary member of this Bob-owned room. Her706 must not
        // remove Dave immediately; two other real accounts complete the vote.
        let voteRoom = "Vote " + suffix
        _ = try peer(["action": "create-vote", "room": voteRoom])
        tap("rooms.refresh")
        setExpanded("rooms.category.1", true)
        selectRoom(voteRoom)
        selectModeration("kick", user: "dave")
        try assertMember("dave", in: voteRoom, present: true, actor: "bob")
        dismissActionNotice()
        assertRoomText("A vote to kick dave was started by alice")
        _ = try peer(["action": "kick", "actor": "carol", "room": voteRoom, "target": "dave"])
        try assertMember("dave", in: voteRoom, present: true, actor: "bob")
        _ = try peer(["action": "kick", "actor": "hailam", "room": voteRoom, "target": "dave"])
        try assertMember("dave", in: voteRoom, present: false, actor: "bob")
        assertRoomText("dave was kicked by a room vote (3/3)")
        capture("\(mode)-feature-ordinary-vote-kick")
        leaveCurrentRoom(voteRoom)
        if theme == "classic-v3" { tap("rooms.back") } else { tap("tab.friends") }

        tap("friends.contact.bob")
        sendFeatureText("Hello from iOS " + suffix, theme: theme)
        assertReply("Hello from iOS " + suffix)
        tap("command.menu"); tap("chat.action.block"); tap("action.confirm")
        dismissActionNotice()
        let hidden = "Hidden after block " + suffix
        _ = try peer(["action": "private-send", "body": hidden])
        let hiddenRow = app.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", hidden)).firstMatch
        let suppressed = XCTNSPredicateExpectation(predicate: NSPredicate(format: "exists == true"), object: hiddenRow)
        suppressed.isInverted = true
        XCTAssertEqual(XCTWaiter.wait(for: [suppressed], timeout: 2), .completed, "Personal block leaked Bob's incoming text")
        tap("command.menu")
        if theme == "classic-v3" { tap("chat.more") }
        tap("chat.action.blocks")
        _ = wait("page.screen")
        assertRoomText("bob")
        capture("\(mode)-feature-original-block-list")
        pageButton("Unblock")
        assertRoomText("You have no blocked users.")
        closeOriginalPage()
        let restored = "Visible after unblock " + suffix
        _ = try peer(["action": "private-send", "body": restored])
        assertRoomText(restored)
        capture("\(mode)-feature-unblock-delivery-restored")

        _ = try peer(["action": "photo-send", "transport": mode])
        let photo = app.descendants(matching: .any).matching(NSPredicate(format: "identifier BEGINSWITH %@", "chat.photo.")).firstMatch
        XCTAssertTrue(photo.waitForExistence(timeout: 20), "Original500 photo did not render in the transcript")
        reveal(photo)
        XCTAssertTrue(photo.isHittable)
        photo.tap()
        _ = wait("photo.preview")
        capture("\(mode)-feature-original-photo-preview")
        tap("chat.sheet.close")
        try exerciseComposer(theme: theme)

        let previous = Set(try peer(["action": "groups"])["groups"] as? [String] ?? [])
        inviteFeatureUser("carol", theme: theme)
        var group = ""
        for _ in 0..<40 {
            let current = try peer(["action": "groups"])["groups"] as? [String] ?? []
            if let created = current.first(where: { !previous.contains($0) }) { group = created; break }
            pauseForDelivery()
        }
        XCTAssertFalse(group.isEmpty, "Original750/group500 did not create a private group")
        _ = wait("tab.group:\(group)")
        inviteFeatureUser("dave", theme: theme)
        let members = try peer(["action": "group", "id": group])["participants"] as? [[String: Any]] ?? []
        XCTAssertEqual(Set(members.compactMap { $0["username"] as? String }), Set(["alice", "bob", "carol", "dave"]))
        tap("command.menu")
        if theme == "classic-v3" { tap("chat.more") }
        tap("group.participants")
        _ = wait("participant.select.dave")
        capture("\(mode)-feature-group-participants")
        tap("participant.back")
        sendFeatureText("Hello group from iOS " + suffix, theme: theme)
        assertRoomText("Group reply: Hello group from iOS " + suffix)
        capture("\(mode)-feature-group-reply")
        tap("command.menu"); tap("group.leave"); tap("group.leave.confirm")
        _ = wait("friends.screen")
        XCTAssertFalse(element("tab.group:\(group)").exists, "Leaving retained the private-group tab")
        let remaining = try peer(["action": "group", "id": group])["participants"] as? [[String: Any]] ?? []
        XCTAssertFalse(remaining.contains { ($0["username"] as? String) == "alice" }, "Original753 did not remove Alice's membership")
        logout("\(mode)-features")
    }

    private func exerciseLiveThemeChange(replyTo message: String) {
        let draft = "Unsent theme switch draft"
        editor().tap()
        editor().typeText(draft)
        chooseTheme("classic", fromMenu: true)
        _ = wait("chat.screen")
        _ = wait("chat.write")
        assertReply(message, timeout: 5)
        capture("http-classic-conversation-retained")

        chooseTheme("modern", fromMenu: true)
        _ = wait("chat.screen")
        assertReply(message, timeout: 5)
        XCTAssertEqual(editor().value as? String, draft, "Theme switching lost the unsent draft")
        capture("http-modern-draft-retained")
        editor().tap()
        editor().typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: draft.count))
    }

    private func exerciseV3ThemeChange(replyTo message: String) {
        let draft = "Draft from Classic v3"
        tap("chat.write")
        editor().tap()
        editor().typeText(draft)
        tap("chat.back")
        chooseTheme("modern", fromMenu: true)
        _ = wait("glass.native")
        assertReply(message, timeout: 5)
        XCTAssertEqual(editor().value as? String, draft, "v3 to Modern lost the unsent draft")
        chooseTheme("classic", fromMenu: true)
        assertReply(message, timeout: 5)
        chooseTheme("classic-v3", fromMenu: true)
        assertReply(message, timeout: 5)
        capture("v3-conversation-retained")
        tap("chat.write")
        XCTAssertEqual(editor().value as? String, draft, "Switching all three themes lost the v3 draft")
        capture("v3-draft-retained")
        editor().tap()
        editor().typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: draft.count))
        tap("chat.back")
    }

    private func exercise(_ mode: String, theme: String, verifyThemeLifecycle: Bool = true) throws {
        chooseTheme(theme)
        // Theme storage must survive an actual app process restart. Session
        // credentials remain unchecked until the separate Keychain acceptance.
        if verifyThemeLifecycle {
            try relaunch()
            loginScreen()
        }
        _ = wait("theme.\(theme)")
        capture("\(mode)-\(theme)-login\(verifyThemeLifecycle ? "-restored" : "")")
        let port = try XCTUnwrap(environment["MIG33_SMOKE_\(mode.uppercased())_PORT"])
        configure(mode, port: port)
        fill("login.username", "alice")
        fill("login.password", "alice-demo", secure: true)
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        _ = wait("friends.contact.bob")
        if theme == "modern" { _ = wait("glass.native") }
        capture("\(mode)-\(theme)-friends")
        tap("friends.contact.bob")
        _ = wait("chat.screen")
        capture("\(mode)-\(theme)-conversation-empty")
        captureTree("\(mode)-\(theme)-conversation-empty-tree")
        if theme != "modern" { tap("chat.write") }
        let message = "Hello from iOS \(mode.uppercased())"
        editor().tap()
        editor().typeText(message)
        capture("\(mode)-\(theme)-compose-message")
        tap("chat.send")
        _ = wait("chat.screen")
        assertReply(message)
        capture("\(mode)-\(theme)-conversation-reply")
        captureTree("\(mode)-\(theme)-conversation-reply-tree")
        if theme == "modern" && !verifyThemeLifecycle {
            // Exercise the chat menu before returning to Friends; each route
            // must expose a working menu independently of the other route.
            tap("command.menu")
            _ = wait("appearance.open")
            capture("\(mode)-\(theme)-conversation-menu")
            captureTree("\(mode)-\(theme)-conversation-menu-tree")
            // SwiftUI menus have no synthetic Cancel row. Exercise an actual
            // nonmutating destination and return through its native Back.
            tap("appearance.open")
            _ = wait("appearance.screen")
            tap("appearance.back")
            _ = wait("chat.screen")
            assertReply(message, timeout: 5)
        }
        if theme == "modern" && verifyThemeLifecycle { exerciseLiveThemeChange(replyTo: message) }
        if theme == "classic-v3" && verifyThemeLifecycle { exerciseV3ThemeChange(replyTo: message) }
        logout(mode)
    }

    private func logout(_ name: String, verifyPresentation: Bool = false) {
        tap("tab.friends")
        _ = wait("friends.screen")
        let modern = element("theme.modern").exists
        tap("command.menu")
        if verifyPresentation { XCTAssertEqual(wait("menu.exit").label, "Log out", "Signed-in menu still describes exiting the application") }
        capture("\(name)-menu")
        tap("menu.exit")
        if verifyPresentation {
            let confirm = wait("dialog.confirm")
            let cancel = wait("dialog.cancel")
            XCTAssertEqual(confirm.label, "Log out")
            XCTAssertEqual(cancel.label, "Cancel")
            if modern {
                XCTAssertTrue(app.alerts.firstMatch.exists, "Modern logout confirmation is not a native alert")
            }
            for control in [confirm, cancel] {
                XCTAssertGreaterThanOrEqual(control.frame.height, 44, "Logout action is smaller than its required touch height")
                XCTAssertGreaterThanOrEqual(control.frame.width, 44, "Logout action is smaller than its required touch width")
            }
        }
        capture("\(name)-logout-confirmation")
        if verifyPresentation {
            tap("dialog.cancel")
            _ = wait("friends.screen")
            _ = wait("friends.contact.bob")
            XCTAssertFalse(element("login.submit").exists, "Cancel logged out the active session")
            capture("\(name)-logout-cancel-retained-session")
            tap("command.menu"); tap("menu.exit")
        }
        tap("dialog.confirm")
        loginScreen()
    }

    private func exerciseRememberedLogin() throws {
        // Both core transport passes leave Remember/Auto unchecked. Saving a
        // login must survive a process restart, not merely a React rerender.
        fill("login.password", "alice-demo", secure: true)
        toggleLoginPreference("login.remember")
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        logout("remember-saved")
        try relaunch()
        loginScreen()
        XCTAssertEqual(app.textFields["login.username"].value as? String, "alice")
        let password = app.secureTextFields["login.password"].value as? String ?? ""
        XCTAssertFalse(password.isEmpty, "Keychain password did not survive relaunch")
        capture("remember-restored")

        // Authenticate using only the restored password, then verify Auto Login
        // uses that same saved server/transport after a second process restart.
        toggleLoginPreference("login.auto")
        tap("login.submit")
        _ = wait("friends.screen", timeout: 35)
        logout("remember-auto-saved")
        try relaunch()
        _ = wait("friends.screen", timeout: 45)
        _ = wait("friends.contact.bob")
        capture("remember-auto-login")
        logout("remember-auto")

        // Unchecking Remember must delete the Keychain record and disable Auto.
        toggleLoginPreference("login.remember")
        capture("remember-disabled")
        try relaunch()
        loginScreen()
        XCTAssertEqual(app.textFields["login.username"].value as? String ?? "", "")
        XCTAssertEqual(app.secureTextFields["login.password"].value as? String ?? "", "")
        capture("remember-deleted")
    }

    private func exerciseSecureRejections() throws {
        chooseTheme("modern")
        tap("login.connection")
        XCTAssertFalse(element("connection.http").exists)
        XCTAssertFalse(element("connection.tcp").exists)
        tap("connection.manual")
        for name in ["https-wrong-host", "https-expired", "https-untrusted", "https-downgrade", "https-plaintext",
                     "tls-wrong-host", "tls-expired", "tls-untrusted", "tls-plaintext"] {
            let mode = String(name.split(separator: "-")[0])
            selectTransport(mode)
            fill("connection.host", "127.0.0.1")
            let key = "MIG33_SMOKE_BAD_" + name.uppercased().replacingOccurrences(of: "-", with: "_") + "_PORT"
            fill("connection.port", try XCTUnwrap(environment[key]))
            tap("connection.save")
            _ = wait("dialog.ok", timeout: 25)
            XCTAssertFalse(element("friends.screen").exists, "Rejected endpoint authenticated a session")
            capture("secure-rejected-\(name)")
            tap("dialog.ok")
            _ = wait("connection.save")
        }
        // A valid connection probe must recover after every rejection, using
        // the same secure selector and native trust policy.
        selectTransport("https")
        fill("connection.port", try XCTUnwrap(environment["MIG33_SMOKE_HTTPS_PORT"]))
        tap("connection.save")
        loginScreen()
    }

    func testOriginalHTTPAndTCP() throws {
        loginScreen()
        capture("login")
        if environment["MIG33_SMOKE_SECURE_ONLY"] == "1" { try exerciseSecureRejections() }
        if environment["MIG33_SMOKE_COMMANDS_ONLY"] == "1" {
            try exerciseCommands("https", theme: "modern")
            try exerciseCommands("tls", theme: "classic-v3")
            capture("commands-logged-out")
            return
        }
        if environment["MIG33_SMOKE_MODERN_ONLY"] == "1" {
            let previousMotion = try setSystemReduceMotion(false)
            let previousTransparency = try setSystemAccessibility(.transparency, false)
            let previousContrast = try setSystemAccessibility(.contrast, false)
            defer {
                _ = try? setSystemAccessibility(.contrast, previousContrast)
                _ = try? setSystemAccessibility(.transparency, previousTransparency)
                _ = try? setSystemReduceMotion(previousMotion)
            }
            try exerciseModernPresentation()
            _ = try setSystemAccessibility(.transparency, true)
            try exerciseModernAccessibility("reduce-transparency", opaque: true)
            _ = try setSystemAccessibility(.transparency, false)
            _ = try setSystemAccessibility(.contrast, true)
            try exerciseModernAccessibility("increase-contrast", opaque: false)
            capture("modern-logged-out")
            return
        }
        if environment["MIG33_SMOKE_UNREAD_ONLY"] == "1" {
            let originalReduceMotion = try setSystemReduceMotion(false)
            defer { _ = try? setSystemReduceMotion(originalReduceMotion) }
            try exerciseUnread("https")
            _ = try setSystemReduceMotion(true)
            try exerciseUnread("https", steady: true)
            _ = try setSystemReduceMotion(false)
            try exerciseUnread("tls")
            capture("unread-logged-out")
            return
        }
        if environment["MIG33_SMOKE_FEATURES_ONLY"] == "1" {
            try exerciseFeatures("https", theme: "modern")
            try exerciseFeatures("tls", theme: "classic-v3")
            capture("features-logged-out")
            return
        }
        if environment["MIG33_SMOKE_ROOMS_ONLY"] == "1" {
            try exerciseRooms("https", theme: "modern")
            try exerciseRooms("tls", theme: "classic-v3")
            capture("rooms-logged-out")
            return
        }
        if environment["MIG33_SMOKE_VISUAL_ONLY"] == "1" {
            try exercise("https", theme: "modern", verifyThemeLifecycle: false)
            capture("logged-out")
            return
        }
        if environment["MIG33_SMOKE_V3_ONLY"] == "1" {
            try exercise("https", theme: "classic-v3")
            try exercise("tls", theme: "classic-v3", verifyThemeLifecycle: false)
            capture("v3-logged-out")
            return
        }
        try exercise("https", theme: "modern")
        try exercise("tls", theme: "classic")
        try exercise("https", theme: "classic-v3")
        chooseTheme("classic")
        try exerciseRememberedLogin()
        chooseTheme("modern")
        capture("logged-out")
    }
}
