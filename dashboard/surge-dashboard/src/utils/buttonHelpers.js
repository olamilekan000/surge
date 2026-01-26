export const getPauseResumeButtonProps = (isPaused, actionLoading) => {
  if (actionLoading) {
    return {
      text: isPaused ? "Resuming..." : "Pausing...",
      color: isPaused ? "success" : "warning",
      icon: "loading",
    };
  }

  return {
    text: isPaused ? "Resume Queue" : "Pause Queue",
    color: isPaused ? "success" : "warning",
    icon: isPaused ? "play" : "pause",
  };
};
