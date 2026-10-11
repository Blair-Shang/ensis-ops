#pragma once

#include "ensisops/sqllsp/parser.hpp"

#include <string>
#include <vector>

namespace ensisops::oracle::lsp {

/** Oracle 启发式 DialectParser（对齐达梦早期补全路径）。 */
class OracleParser final : public ensisops::sqllsp::DialectParser {
 public:
  std::vector<std::string> Keywords() const override;
  std::vector<std::string> Functions() const override;
  ensisops::sqllsp::CompletionContext CompletionContext(const std::string& text,
                                                     ensisops::sqllsp::Position pos) const override;
  std::string QuoteIdent(const std::string& name) const override;
};

}  // namespace ensisops::oracle::lsp
