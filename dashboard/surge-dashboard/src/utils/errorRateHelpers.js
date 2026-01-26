export const calculateErrorRate = (processed, failed) => {
  const total = processed + failed;
  return total > 0 ? ((failed / total) * 100).toFixed(2) : "0.00";
};

export const getErrorRateSeverity = (errorRate) => {
  const rate = parseFloat(errorRate);
  if (rate > 10) return { label: "High", color: "error" };
  if (rate > 5) return { label: "Medium", color: "warning" };
  return { label: "Low", color: "success" };
};
