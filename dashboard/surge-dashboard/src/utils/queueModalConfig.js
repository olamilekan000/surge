export const getQueueModalConfig = (action, queueName) => {
  const configs = {
    pause: {
      title: "Pause Queue",
      message: `Are you sure you want to pause the queue "${queueName}"? This will stop processing new jobs.`,
      confirmText: "Pause",
      confirmColor: "warning",
    },
    resume: {
      title: "Resume Queue",
      message: `Are you sure you want to resume the queue "${queueName}"? This will start processing jobs again.`,
      confirmText: "Resume",
      confirmColor: "primary",
    },
    drain: {
      title: "Drain Queue",
      message: `Are you sure you want to drain all pending jobs from "${queueName}"? This action cannot be undone.`,
      confirmText: "Drain",
      confirmColor: "error",
    },
    default: {
      title: "Confirm Action",
      message: "Are you sure you want to continue?",
      confirmText: "Confirm",
      confirmColor: "primary",
    },
  };

  return configs[action] || configs.default;
};
