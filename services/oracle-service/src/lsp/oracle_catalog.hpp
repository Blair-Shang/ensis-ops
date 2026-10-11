#pragma once

#include "ensisops/sqllsp/catalog.hpp"
#include "session/manager.hpp"

namespace ensisops::oracle::lsp {

/** 进程内复用 catalog::List* / tree 例程列表。 */
class OracleCatalog final : public ensisops::sqllsp::RoutineCatalog {
 public:
  explicit OracleCatalog(session::Manager& sessions) : sessions_(sessions) {}

  bool ListSchemas(const ensisops::sqllsp::CatalogParams& params,
                   std::vector<ensisops::sqllsp::SchemaHit>& out, bool& truncated,
                   std::string& error) override;
  bool ListTables(const ensisops::sqllsp::CatalogParams& params,
                  std::vector<ensisops::sqllsp::TableHit>& out, bool& truncated,
                  std::string& error) override;
  bool ListColumns(const ensisops::sqllsp::CatalogParams& params,
                   std::vector<ensisops::sqllsp::ColumnHit>& out, bool& truncated,
                   std::string& error) override;
  bool ListRoutines(const ensisops::sqllsp::CatalogParams& params,
                    std::vector<ensisops::sqllsp::RoutineHit>& out, bool& truncated,
                    std::string& error) override;

 private:
  session::Manager& sessions_;
};

}  // namespace ensisops::oracle::lsp
