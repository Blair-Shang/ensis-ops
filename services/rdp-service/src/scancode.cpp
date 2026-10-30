#include "scancode.hpp"

#include <string>
#include <unordered_map>

namespace ensisops::rdp {
namespace {

constexpr std::uint32_t kExtended = 0x100;

const std::unordered_map<std::string, std::uint32_t>& Table() {
  static const std::unordered_map<std::string, std::uint32_t> table = {
      {"Escape", 0x01},
      {"Digit1", 0x02},
      {"Digit2", 0x03},
      {"Digit3", 0x04},
      {"Digit4", 0x05},
      {"Digit5", 0x06},
      {"Digit6", 0x07},
      {"Digit7", 0x08},
      {"Digit8", 0x09},
      {"Digit9", 0x0A},
      {"Digit0", 0x0B},
      {"Minus", 0x0C},
      {"Equal", 0x0D},
      {"Backspace", 0x0E},
      {"Tab", 0x0F},
      {"KeyQ", 0x10},
      {"KeyW", 0x11},
      {"KeyE", 0x12},
      {"KeyR", 0x13},
      {"KeyT", 0x14},
      {"KeyY", 0x15},
      {"KeyU", 0x16},
      {"KeyI", 0x17},
      {"KeyO", 0x18},
      {"KeyP", 0x19},
      {"BracketLeft", 0x1A},
      {"BracketRight", 0x1B},
      {"Enter", 0x1C},
      {"ControlLeft", 0x1D},
      {"KeyA", 0x1E},
      {"KeyS", 0x1F},
      {"KeyD", 0x20},
      {"KeyF", 0x21},
      {"KeyG", 0x22},
      {"KeyH", 0x23},
      {"KeyJ", 0x24},
      {"KeyK", 0x25},
      {"KeyL", 0x26},
      {"Semicolon", 0x27},
      {"Quote", 0x28},
      {"Backquote", 0x29},
      {"ShiftLeft", 0x2A},
      {"Backslash", 0x2B},
      {"KeyZ", 0x2C},
      {"KeyX", 0x2D},
      {"KeyC", 0x2E},
      {"KeyV", 0x2F},
      {"KeyB", 0x30},
      {"KeyN", 0x31},
      {"KeyM", 0x32},
      {"Comma", 0x33},
      {"Period", 0x34},
      {"Slash", 0x35},
      {"ShiftRight", 0x36},
      {"AltLeft", 0x38},
      {"Space", 0x39},
      {"CapsLock", 0x3A},
      {"F1", 0x3B},
      {"F2", 0x3C},
      {"F3", 0x3D},
      {"F4", 0x3E},
      {"F5", 0x3F},
      {"F6", 0x40},
      {"F7", 0x41},
      {"F8", 0x42},
      {"F9", 0x43},
      {"F10", 0x44},
      {"NumLock", 0x45},
      {"ScrollLock", 0x46},
      {"Numpad7", 0x47},
      {"Numpad8", 0x48},
      {"Numpad9", 0x49},
      {"NumpadSubtract", 0x4A},
      {"Numpad4", 0x4B},
      {"Numpad5", 0x4C},
      {"Numpad6", 0x4D},
      {"NumpadAdd", 0x4E},
      {"Numpad1", 0x4F},
      {"Numpad2", 0x50},
      {"Numpad3", 0x51},
      {"Numpad0", 0x52},
      {"NumpadDecimal", 0x53},
      {"IntlBackslash", 0x56},
      {"F11", 0x57},
      {"F12", 0x58},
      {"NumpadEnter", 0x1C | kExtended},
      {"ControlRight", 0x1D | kExtended},
      {"NumpadDivide", 0x35 | kExtended},
      {"AltRight", 0x38 | kExtended},
      {"Home", 0x47 | kExtended},
      {"ArrowUp", 0x48 | kExtended},
      {"PageUp", 0x49 | kExtended},
      {"ArrowLeft", 0x4B | kExtended},
      {"ArrowRight", 0x4D | kExtended},
      {"End", 0x4F | kExtended},
      {"ArrowDown", 0x50 | kExtended},
      {"PageDown", 0x51 | kExtended},
      {"Insert", 0x52 | kExtended},
      {"Delete", 0x53 | kExtended},
      {"MetaLeft", 0x5B | kExtended},
      {"MetaRight", 0x5C | kExtended},
  };
  return table;
}

}  // namespace

bool LookupScancode(std::string_view code, std::uint32_t& scancode) {
  const auto& table = Table();
  const auto it = table.find(std::string(code));
  if (it == table.end()) {
    return false;
  }
  scancode = it->second;
  return true;
}

}  // namespace ensisops::rdp
