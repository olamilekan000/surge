import {
  Box,
  Typography,
  Skeleton,
  FormControl,
  Select,
  MenuItem,
  Pagination,
} from "@mui/material";
import { QueueTable } from "./QueueTable";

export function QueueTableContent({
  loading,
  filteredQueues,
  paginatedQueues,
  queuesWithStats,
  selectedNs,
  namespaces,
  onStatsUpdate,
  queuePage,
  queuePageSize,
  totalQueuePages,
  onPageChange,
  onPageSizeChange,
}) {
  if (loading) {
    return (
      <Box>
        {[1, 2, 3, 4, 5].map((i) => (
          <Box key={i} sx={{ display: "flex", gap: 2, mb: 2 }}>
            <Skeleton variant="rectangular" width="100%" height={60} />
          </Box>
        ))}
      </Box>
    );
  }

  if (filteredQueues.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 8 }}>
        <Typography variant="h6" color="text.secondary" sx={{ mb: 1 }}>
          No Queues Found
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {getEmptyMessage(selectedNs, namespaces.length)}
        </Typography>
      </Box>
    );
  }

  return (
    <>
      <QueueTable queues={paginatedQueues} onStatsUpdate={onStatsUpdate} />
      {queuesWithStats.length > 0 && (
        <QueueTablePagination
          queuePage={queuePage}
          queuePageSize={queuePageSize}
          totalItems={queuesWithStats.length}
          totalPages={totalQueuePages}
          onPageChange={onPageChange}
          onPageSizeChange={onPageSizeChange}
        />
      )}
    </>
  );
}

function getEmptyMessage(selectedNs, namespaceCount) {
  const baseMessage = `No queues found in namespace "${selectedNs}".`;
  const suggestion =
    namespaceCount > 1
      ? "Try selecting a different namespace."
      : "Start enqueueing jobs to see them here.";
  return `${baseMessage} ${suggestion}`;
}

function QueueTablePagination({
  queuePage,
  queuePageSize,
  totalItems,
  totalPages,
  onPageChange,
  onPageSizeChange,
}) {
  return (
    <Box
      sx={{
        mt: 3,
        pt: 2,
        borderTop: 1,
        borderColor: "divider",
        display: "flex",
        flexDirection: { xs: "column", sm: "row" },
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Showing <strong>{(queuePage - 1) * queuePageSize + 1}</strong> to{" "}
          <strong>{Math.min(queuePage * queuePageSize, totalItems)}</strong> of{" "}
          <strong>{totalItems}</strong> results
        </Typography>
        <PageSizeSelector
          value={queuePageSize}
          onChange={onPageSizeChange}
        />
      </Box>
      {totalPages > 1 && (
        <Pagination
          count={totalPages}
          page={queuePage}
          onChange={onPageChange}
          color="primary"
          size="small"
          showFirstButton
          showLastButton
        />
      )}
    </Box>
  );
}

function PageSizeSelector({ value, onChange }) {
  return (
    <FormControl size="small" sx={{ minWidth: 100 }}>
      <Select value={value} onChange={(e) => onChange(Number(e.target.value))}>
        <MenuItem value={10}>10</MenuItem>
        <MenuItem value={25}>25</MenuItem>
        <MenuItem value={50}>50</MenuItem>
        <MenuItem value={100}>100</MenuItem>
      </Select>
    </FormControl>
  );
}
