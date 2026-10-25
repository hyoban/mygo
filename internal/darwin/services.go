//go:build darwin

package darwin

import (
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// The surface's view is a requestor of the Services menu (NSServicesMenuRequestor)
// for the text the content selects, which TextServices give: AppKit lists
// the services and Writing Tools that take it in the app's Services menu
// and in context menus of text (PopupMenu), as for its own text views.

// stringPasteboardType is NSPasteboardTypeString.
const stringPasteboardType = "public.utf8-plain-text"

// servicesRegistered is whether the app told AppKit of the types its
// views give services and take back.
var servicesRegistered bool

func (s *surface) SetTextServices(t platform.TextServices) {
	s.services = t
	if !servicesRegistered {
		servicesRegistered = true
		withPool(func() {
			types := nsArray(nsString(stringPasteboardType))
			send(s.w.b.app, "registerServicesMenuSendTypes:returnTypes:", uintptr(types), uintptr(types))
		})
	}
}

// isString reports whether a pasteboard type is plain text, or none
// (0), as validRequestorForSendType:returnType: asks of either.
func isString(t id) bool {
	if t == 0 {
		return true
	}
	switch goString(t) {
	case stringPasteboardType, "NSStringPboardType":
		return true
	}
	return false
}

func serviceViewMethods() []objc.MethodDef {
	return []objc.MethodDef{
		method("validRequestorForSendType:returnType:", func(self id, cmd objc.SEL, sendType, returnType id) id {
			if s := theBackend.surfaceOf(self); s != nil && s.services != nil && isString(sendType) && isString(returnType) {
				text, editable := s.services.ServicedText()
				if (sendType == 0 || text != "") && (returnType == 0 || editable) {
					return self
				}
			}
			return sendSuper(self, "MyGoSurfaceView", cmd, uintptr(sendType), uintptr(returnType))
		}),
		method("writeSelectionToPasteboard:types:", func(self id, _ objc.SEL, board, types id) bool {
			s := theBackend.surfaceOf(self)
			if s == nil || s.services == nil {
				return false
			}
			text, _ := s.services.ServicedText()
			if text == "" {
				return false
			}
			send(board, "clearContents")
			return sendBool(board, "setString:forType:", uintptr(nsString(text)), uintptr(nsString(stringPasteboardType)))
		}),
		method("readSelectionFromPasteboard:", func(self id, _ objc.SEL, board id) bool {
			s := theBackend.surfaceOf(self)
			if s == nil || s.services == nil {
				return false
			}
			text := send(board, "stringForType:", uintptr(nsString(stringPasteboardType)))
			if text == 0 {
				return false
			}
			s.services.ReplaceServicedText(goString(text))
			return true
		}),
	}
}
