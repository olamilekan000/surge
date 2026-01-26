import React, { useState } from "react";
import toast from "react-hot-toast";
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
  Checkbox,
  Button,
  CircularProgress,
  Stack,
  Alert,
  Paper,
  Grid,
} from "@mui/material";
import {
  Error as ErrorIcon,
  Replay as RetryIcon,
  ExpandMore as ExpandMoreIcon,
  Description as DescriptionIcon,
} from "@mui/icons-material";
import { api } from "../../services/api";
import { ConfirmationModal } from "../ConfirmationModal";

export function FailedJobsTable({
  jobs,
  selectedJobs,
  onToggleSelection,
  onSelectAll,
  onRetry,
}) {
  const allSelected = jobs.length > 0 && selectedJobs.size === jobs.length;
  const someSelected = selectedJobs.size > 0 && selectedJobs.size < jobs.length;

  return (
    <TableContainer>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell padding="checkbox">
              <Checkbox
                checked={allSelected}
                indeterminate={someSelected}
                onChange={onSelectAll}
                color="primary"
              />
            </TableCell>
            <TableCell>Job ID</TableCell>
            <TableCell>Topic</TableCell>
            <TableCell>Error</TableCell>
            <TableCell>Retries</TableCell>
            <TableCell>Failed At</TableCell>
            <TableCell align="right">Actions</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {jobs.map((job, idx) => {
            const jobId = job.id || `job-${idx}`;
            const isSelected = selectedJobs.has(jobId);
            return (
              <FailedJobRow
                key={jobId}
                job={job}
                isSelected={isSelected}
                onToggleSelection={() => onToggleSelection(jobId)}
                onRetry={onRetry}
              />
            );
          })}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function FailedJobRow({ job, isSelected, onToggleSelection, onRetry }) {
  const [retrying, setRetrying] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  if (!job) return null;

  const jobId = job.id || "Unknown";
  const retries = job.retry_count ?? 0;
  const maxRetries = job.max_retries ?? 0;
  const error = job.last_error ?? "Unknown error";
  const topic = job.topic || "N/A";
  const createdAt = job.created_at;
  const completedAt = job.completed_at;
  const failedAt = completedAt || createdAt;

  const handleRetryClick = () => {
    if (!jobId || jobId === "Unknown") {
      toast.error("Cannot retry: Job ID is unknown");
      return;
    }
    setShowConfirm(true);
  };

  const handleRetry = async () => {
    setRetrying(true);
    try {
      await api.retryJob(jobId);
      toast.success("Job queued for retry!");
      if (onRetry) onRetry();
      setShowConfirm(false);
    } catch (err) {
      toast.error("Failed to retry: " + err.message);
    } finally {
      setRetrying(false);
    }
  };

  const truncatedError =
    error.length > 80 ? error.substring(0, 80) + "..." : error;

  return (
    <>
      <TableRow
        hover
        selected={isSelected}
        sx={{
          backgroundColor: isSelected ? "action.selected" : "inherit",
        }}
      >
        <TableCell padding="checkbox">
          <Checkbox
            checked={isSelected}
            onChange={onToggleSelection}
            color="primary"
          />
        </TableCell>
        <TableCell>
          <Typography variant="body2" fontFamily="monospace">
            {jobId}
          </Typography>
        </TableCell>
        <TableCell>
          <Typography variant="body2">{topic}</Typography>
        </TableCell>
        <TableCell>
          <Typography
            variant="body2"
            sx={{
              maxWidth: 400,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
            title={error}
          >
            {truncatedError}
          </Typography>
        </TableCell>
        <TableCell>
          <Chip
            label={maxRetries > 0 ? `${retries}/${maxRetries}` : `${retries}/∞`}
            color={retries > maxRetries && maxRetries > 0 ? "error" : "warning"}
            size="small"
            sx={{ fontWeight: 500 }}
          />
        </TableCell>
        <TableCell>
          <Typography variant="body2" color="text.secondary">
            {failedAt ? new Date(failedAt).toLocaleString() : "N/A"}
          </Typography>
        </TableCell>
        <TableCell align="right">
          <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
            <Button
              onClick={handleRetryClick}
              disabled={retrying}
              variant="contained"
              size="small"
              startIcon={
                retrying ? (
                  <CircularProgress size={12} color="inherit" />
                ) : (
                  <RetryIcon />
                )
              }
            >
              {retrying ? "Retrying..." : "Retry"}
            </Button>
            <Button
              onClick={() => setExpanded(!expanded)}
              variant="outlined"
              size="small"
              endIcon={
                expanded ? (
                  <ExpandMoreIcon />
                ) : (
                  <ExpandMoreIcon sx={{ transform: "rotate(180deg)" }} />
                )
              }
            >
              {expanded ? "Hide" : "Details"}
            </Button>
          </Box>
        </TableCell>
      </TableRow>
      {expanded && (
        <TableRow>
          <TableCell colSpan={7} sx={{ py: 2, bgcolor: "grey.50" }}>
            <JobDetailsExpanded job={job} />
          </TableCell>
        </TableRow>
      )}

      <ConfirmationModal
        isOpen={showConfirm}
        onClose={() => setShowConfirm(false)}
        onConfirm={handleRetry}
        title="Retry Failed Job"
        message={`Are you sure you want to retry job "${jobId}"? This will move it back to the queue for processing.`}
        confirmText="Retry"
        confirmColor="primary"
        isLoading={retrying}
      />
    </>
  );
}

function JobDetailsExpanded({ job }) {
  if (!job) return null;

  const error = job.last_error;
  const payload = job.args ?? {};
  const createdAt = job.created_at;
  const completedAt = job.completed_at;

  return (
    <Stack spacing={2}>
      {error && (
        <Box>
          <Typography
            variant="subtitle2"
            sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}
          >
            <ErrorIcon color="error" sx={{ fontSize: 18 }} />
            Error Message
          </Typography>
          <Alert
            severity="error"
            sx={{
              "& .MuiAlert-message": {
                fontFamily: "monospace",
                fontSize: "0.75rem",
              },
            }}
          >
            {error}
          </Alert>
        </Box>
      )}
      <Box>
        <Typography
          variant="subtitle2"
          sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}
        >
          <DescriptionIcon sx={{ fontSize: 18, color: "text.secondary" }} />
          Job Payload
        </Typography>
        <Paper
          variant="outlined"
          sx={{
            p: 2,
            bgcolor: "grey.50",
            maxHeight: 384,
            overflow: "auto",
            "& pre": {
              margin: 0,
              fontFamily: "monospace",
              fontSize: "0.75rem",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
            },
          }}
        >
          <pre>
            {typeof payload === "string"
              ? payload
              : JSON.stringify(payload, null, 2)}
          </pre>
        </Paper>
      </Box>
      <Grid container spacing={2}>
        {createdAt && (
          <Grid item xs={6}>
            <Typography variant="body2" color="text.secondary">
              Created: <strong>{new Date(createdAt).toLocaleString()}</strong>
            </Typography>
          </Grid>
        )}
        {completedAt && (
          <Grid item xs={6}>
            <Typography variant="body2" color="text.secondary">
              Failed: <strong>{new Date(completedAt).toLocaleString()}</strong>
            </Typography>
          </Grid>
        )}
      </Grid>
    </Stack>
  );
}
