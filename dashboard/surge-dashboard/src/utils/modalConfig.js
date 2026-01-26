export const getModalConfig = (type, queue, data) => {
  const configs = {
    pause: {
      title: "Pause Queue",
      message: `Are you sure you want to pause the queue "${queue}"? This will stop processing new jobs.`,
      confirmText: "Pause",
      confirmColor: "amber",
    },
    resume: {
      title: "Resume Queue",
      message: `Are you sure you want to resume the queue "${queue}"? This will start processing jobs again.`,
      confirmText: "Resume",
      confirmColor: "blue",
    },
    bulkRetry: {
      title: "Retry Selected Jobs",
      message: `Are you sure you want to retry ${data?.count || 0} selected job${data?.count !== 1 ? "s" : ""}? This will move them back to the queue for processing.`,
      confirmText: "Retry",
      confirmColor: "blue",
    },
    default: {
      title: "Confirm Action",
      message: "Are you sure you want to continue?",
      confirmText: "Confirm",
      confirmColor: "blue",
    },
  };

  return configs[type] || configs.default;
};

export const handleModalConfirm = (type, handlers) => {
  const { handlePauseResume, handleBulkRetry } = handlers;
  
  if (type === "pause" || type === "resume") {
    handlePauseResume();
  } else if (type === "bulkRetry") {
    handleBulkRetry();
  }
};
