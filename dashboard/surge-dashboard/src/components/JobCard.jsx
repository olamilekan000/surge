import React, { useState } from "react";
import toast from "react-hot-toast";
import {
  Card,
  CardContent,
  Box,
  Typography,
  Chip,
  Alert,
  Button,
  CircularProgress,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Paper,
  Checkbox,
} from "@mui/material";
import {
  Description as DescriptionIcon,
  Replay as RetryIcon,
  Error as ErrorIcon,
  ExpandMore as ExpandMoreIcon,
} from "@mui/icons-material";
import { api } from "../services/api";
import { ConfirmationModal } from "./ConfirmationModal";

export function JobCard({ job, onRetry, selected, onToggleSelection }) {
  const [expanded, setExpanded] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  if (!job) return null;

  const jobId = job.id || "Unknown";
  const retries = job.retry_count ?? 0;
  const maxRetries = job.max_retries ?? 0;
  const error = job.last_error;
  const topic = job.topic;
  const createdAt = job.created_at;
  const payload = job.args ?? {};

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

  return (
    <Card
      sx={{
        border: selected ? 2 : 1,
        borderColor: selected ? "primary.main" : "divider",
        bgcolor: selected ? "action.selected" : "background.paper",
        transition: "all 0.2s",
        "&:hover": {
          boxShadow: 2,
        },
      }}
    >
      <CardContent>
        <Box sx={{ display: "flex", justifyContent: "space-between", gap: 2 }}>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                mb: 2,
                flexWrap: "wrap",
              }}
            >
              {onToggleSelection && (
                <Checkbox
                  checked={selected || false}
                  onChange={onToggleSelection}
                  color="primary"
                  size="small"
                />
              )}
              <DescriptionIcon sx={{ fontSize: 20, color: "text.secondary" }} />
              <Typography
                variant="body2"
                fontFamily="monospace"
                sx={{ fontWeight: 500 }}
              >
                {jobId}
              </Typography>
              {topic && <Chip label={topic} size="small" variant="outlined" />}
              {retries > 0 && (
                <Chip
                  icon={<RetryIcon sx={{ fontSize: 14 }} />}
                  label={`${retries}/${maxRetries > 0 ? maxRetries : "∞"} retries${retries > maxRetries && maxRetries > 0 ? " (exceeded)" : ""}`}
                  size="small"
                  color={
                    retries > maxRetries && maxRetries > 0 ? "error" : "warning"
                  }
                  variant="outlined"
                />
              )}
              {createdAt && (
                <Typography variant="caption" color="text.secondary">
                  {new Date(createdAt).toLocaleString()}
                </Typography>
              )}
            </Box>

            {error && (
              <Alert severity="error" icon={<ErrorIcon />} sx={{ mb: 2 }}>
                <Typography variant="body2" component="div">
                  <strong>Error:</strong> {error}
                </Typography>
              </Alert>
            )}
          </Box>

          <Box sx={{ display: "flex", gap: 1, flexShrink: 0 }}>
            <Button
              onClick={handleRetryClick}
              disabled={retrying}
              variant="contained"
              size="small"
              startIcon={
                retrying ? (
                  <CircularProgress size={16} color="inherit" />
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
        </Box>

        {expanded && (
          <Accordion
            expanded={expanded}
            onChange={() => setExpanded(!expanded)}
          >
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography
                variant="subtitle2"
                sx={{ display: "flex", alignItems: "center", gap: 1 }}
              >
                <DescriptionIcon sx={{ fontSize: 18 }} />
                Job Payload
              </Typography>
            </AccordionSummary>
            <AccordionDetails>
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
            </AccordionDetails>
          </Accordion>
        )}
      </CardContent>

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
    </Card>
  );
}
