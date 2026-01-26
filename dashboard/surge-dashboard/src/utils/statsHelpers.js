export const buildStatsMap = (data) => {
  const statsMap = {};
  data.forEach((item) => {
    statsMap[`${item.namespace}:${item.queue}`] = item.stats;
  });
  return statsMap;
};
