import React from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Box,
  Typography,
  Chip,
} from "@mui/material";
import { AccessTime as AccessTimeIcon } from "@mui/icons-material";

export function ScheduledJobsTable({ jobs }) {
  if (!jobs || jobs.length === 0) return null;

  return (
    <TableContainer>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell>Job ID</TableCell>
            <TableCell>Topic</TableCell>
            <TableCell>Scheduled For</TableCell>
            <TableCell>Remaining</TableCell>
            <TableCell>Created At</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {jobs.map((job, idx) => {
            const jobId = job.id || `job-${idx}`;
            const scheduledAt = job.scheduled_at
              ? new Date(job.scheduled_at)
              : null;
            const now = new Date();
            const remainingMs = scheduledAt ? scheduledAt - now : 0;

            // Format remaining time
            let remainingText = "Ready";
            if (remainingMs > 0) {
              const seconds = Math.floor(remainingMs / 1000);
              const minutes = Math.floor(seconds / 60);
              const hours = Math.floor(minutes / 60);
              const days = Math.floor(hours / 24);

              if (days > 0) remainingText = `in ${days}d ${hours % 24}h`;
              else if (hours > 0)
                remainingText = `in ${hours}h ${minutes % 60}m`;
              else if (minutes > 0)
                remainingText = `in ${minutes}m ${seconds % 60}s`;
              else remainingText = `in ${seconds}s`;
            } else if (scheduledAt) {
              remainingText = "Overdue";
            }

            return (
              <TableRow key={jobId} hover>
                <TableCell>
                  <Typography variant="body2" fontFamily="monospace">
                    {jobId}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{job.topic}</Typography>
                </TableCell>
                <TableCell>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <AccessTimeIcon fontSize="small" color="action" />
                    <Typography variant="body2">
                      {scheduledAt ? scheduledAt.toLocaleString() : "N/A"}
                    </Typography>
                  </Box>
                </TableCell>
                <TableCell>
                  <Chip
                    label={remainingText}
                    size="small"
                    color={remainingMs > 0 ? "primary" : "warning"}
                    variant={remainingMs > 0 ? "outlined" : "filled"}
                    sx={{ fontWeight: 500 }}
                  />
                </TableCell>
                <TableCell>
                  <Typography variant="body2" color="text.secondary">
                    {job.created_at
                      ? new Date(job.created_at).toLocaleString()
                      : "N/A"}
                  </Typography>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
