-- 004_graph_extracted_at.sql
--
-- Marca em compressed_observations o momento em que o knowledge graph
-- já extraiu entidades dela. Permite ao scheduler periódico processar
-- só as observações novas, sem reprocessar (e sem queimar Haiku tokens
-- em loop).
--
-- Linhas pré-existentes ficam com NULL — o filtro IS NULL trata todas
-- elas como "ainda não processadas". Isso é intencional: na primeira
-- rodada após a migration o pipeline vai drenar o backlog histórico,
-- depois converge pra estado estável.

ALTER TABLE compressed_observations ADD COLUMN graph_extracted_at TEXT;
CREATE INDEX IF NOT EXISTS idx_compressed_obs_graph_extracted
  ON compressed_observations(graph_extracted_at)
  WHERE graph_extracted_at IS NULL;
