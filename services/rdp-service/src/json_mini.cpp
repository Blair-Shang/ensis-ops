#include "json_mini.hpp"

#include <cctype>
#include <stdexcept>

namespace ensisops::rdp {
namespace {

class Parser {
 public:
  explicit Parser(std::string_view text) : text_(text) {}

  Json Parse() {
    Skip();
    Json value = ParseValue();
    Skip();
    if (index_ != text_.size()) {
      throw std::runtime_error("invalid request json");
    }
    return value;
  }

 private:
  std::string_view text_;
  std::size_t index_ = 0;

  void Skip() {
    while (index_ < text_.size() &&
           std::isspace(static_cast<unsigned char>(text_[index_]))) {
      ++index_;
    }
  }

  char Peek() const {
    if (index_ >= text_.size()) {
      throw std::runtime_error("invalid request json");
    }
    return text_[index_];
  }

  char Take() {
    const char c = Peek();
    ++index_;
    return c;
  }

  Json ParseValue() {
    Skip();
    const char c = Peek();
    if (c == '{') {
      return ParseObject();
    }
    if (c == '[') {
      return ParseArray();
    }
    if (c == '"') {
      Json value;
      value.type = Json::Type::String;
      value.str = ParseString();
      return value;
    }
    if (c == 't' || c == 'f') {
      return ParseBool();
    }
    if (c == 'n') {
      return ParseNull();
    }
    return ParseNumber();
  }

  Json ParseObject() {
    Take();
    Json value;
    value.type = Json::Type::Object;
    Skip();
    if (Peek() == '}') {
      Take();
      return value;
    }
    while (true) {
      Skip();
      if (Peek() != '"') {
        throw std::runtime_error("invalid request json");
      }
      const std::string key = ParseString();
      Skip();
      if (Take() != ':') {
        throw std::runtime_error("invalid request json");
      }
      value.obj.emplace(key, ParseValue());
      Skip();
      const char sep = Take();
      if (sep == '}') {
        break;
      }
      if (sep != ',') {
        throw std::runtime_error("invalid request json");
      }
    }
    return value;
  }

  Json ParseArray() {
    Take();
    Json value;
    value.type = Json::Type::Array;
    Skip();
    if (Peek() == ']') {
      Take();
      return value;
    }
    while (true) {
      value.arr.push_back(ParseValue());
      Skip();
      const char sep = Take();
      if (sep == ']') {
        break;
      }
      if (sep != ',') {
        throw std::runtime_error("invalid request json");
      }
    }
    return value;
  }

  std::string ParseString() {
    if (Take() != '"') {
      throw std::runtime_error("invalid request json");
    }
    std::string out;
    while (true) {
      if (index_ >= text_.size()) {
        throw std::runtime_error("invalid request json");
      }
      const char c = Take();
      if (c == '"') {
        break;
      }
      if (c != '\\') {
        out.push_back(c);
        continue;
      }
      const char esc = Take();
      switch (esc) {
        case '"':
        case '\\':
        case '/':
          out.push_back(esc);
          break;
        case 'b':
          out.push_back('\b');
          break;
        case 'f':
          out.push_back('\f');
          break;
        case 'n':
          out.push_back('\n');
          break;
        case 'r':
          out.push_back('\r');
          break;
        case 't':
          out.push_back('\t');
          break;
        case 'u': {
          int code = 0;
          for (int i = 0; i < 4; ++i) {
            const char h = Take();
            code <<= 4;
            if (h >= '0' && h <= '9') {
              code += h - '0';
            } else if (h >= 'a' && h <= 'f') {
              code += h - 'a' + 10;
            } else if (h >= 'A' && h <= 'F') {
              code += h - 'A' + 10;
            } else {
              throw std::runtime_error("invalid request json");
            }
          }
          if (code < 0x80) {
            out.push_back(static_cast<char>(code));
          } else if (code < 0x800) {
            out.push_back(static_cast<char>(0xC0 | (code >> 6)));
            out.push_back(static_cast<char>(0x80 | (code & 0x3F)));
          } else {
            out.push_back(static_cast<char>(0xE0 | (code >> 12)));
            out.push_back(static_cast<char>(0x80 | ((code >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (code & 0x3F)));
          }
          break;
        }
        default:
          throw std::runtime_error("invalid request json");
      }
    }
    return out;
  }

  Json ParseBool() {
    Json value;
    value.type = Json::Type::Bool;
    if (text_.substr(index_, 4) == "true") {
      value.boolean = true;
      index_ += 4;
      return value;
    }
    if (text_.substr(index_, 5) == "false") {
      value.boolean = false;
      index_ += 5;
      return value;
    }
    throw std::runtime_error("invalid request json");
  }

  Json ParseNull() {
    if (text_.substr(index_, 4) != "null") {
      throw std::runtime_error("invalid request json");
    }
    index_ += 4;
    Json value;
    value.type = Json::Type::Null;
    return value;
  }

  Json ParseNumber() {
    const std::size_t begin = index_;
    if (Peek() == '-') {
      Take();
    }
    if (index_ >= text_.size() || !std::isdigit(static_cast<unsigned char>(text_[index_]))) {
      throw std::runtime_error("invalid request json");
    }
    while (index_ < text_.size() && std::isdigit(static_cast<unsigned char>(text_[index_]))) {
      ++index_;
    }
    if (index_ < text_.size() && text_[index_] == '.') {
      ++index_;
      while (index_ < text_.size() && std::isdigit(static_cast<unsigned char>(text_[index_]))) {
        ++index_;
      }
    }
    if (index_ < text_.size() && (text_[index_] == 'e' || text_[index_] == 'E')) {
      ++index_;
      if (index_ < text_.size() && (text_[index_] == '+' || text_[index_] == '-')) {
        ++index_;
      }
      while (index_ < text_.size() && std::isdigit(static_cast<unsigned char>(text_[index_]))) {
        ++index_;
      }
    }
    Json value;
    value.type = Json::Type::Number;
    try {
      value.number = std::stod(std::string(text_.substr(begin, index_ - begin)));
    } catch (const std::exception&) {
      throw std::runtime_error("invalid request json");
    }
    return value;
  }
};

}  // namespace

Json Json::Parse(std::string_view text) {
  return Parser(text).Parse();
}

const Json* Json::Get(std::string_view key) const {
  if (type != Type::Object) {
    return nullptr;
  }
  const auto it = obj.find(std::string(key));
  if (it == obj.end()) {
    return nullptr;
  }
  return &it->second;
}

std::string Json::Str(std::string_view key, std::string_view fallback) const {
  const Json* value = Get(key);
  if (value == nullptr || value->type != Type::String) {
    return std::string(fallback);
  }
  return value->str;
}

int Json::Int(std::string_view key, int fallback) const {
  const Json* value = Get(key);
  if (value == nullptr || value->type != Type::Number) {
    return fallback;
  }
  return static_cast<int>(value->number);
}

bool Json::Bool(std::string_view key, bool fallback) const {
  const Json* value = Get(key);
  if (value == nullptr || value->type != Type::Bool) {
    return fallback;
  }
  return value->boolean;
}

}  // namespace ensisops::rdp
