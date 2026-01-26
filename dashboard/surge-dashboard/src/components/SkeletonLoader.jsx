import React from "react";
import { Stack, Box, Skeleton, Grid } from "@mui/material";

export function SkeletonLoader() {
  return (
    <Stack spacing={3}>
      <Box>
        <Skeleton variant="text" width="25%" height={32} sx={{ mb: 1 }} />
        <Skeleton variant="text" width="16%" height={20} />
      </Box>
      <Grid container spacing={3}>
        {[1, 2, 3, 4].map((i) => (
          <Grid item xs={12} sm={6} md={3} key={i}>
            <Skeleton
              variant="rectangular"
              height={128}
              sx={{ borderRadius: 2 }}
            />
          </Grid>
        ))}
      </Grid>
      <Grid container spacing={3}>
        <Grid item xs={12} lg={6}>
          <Skeleton
            variant="rectangular"
            height={160}
            sx={{ borderRadius: 2 }}
          />
        </Grid>
        <Grid item xs={12} lg={6}>
          <Skeleton
            variant="rectangular"
            height={160}
            sx={{ borderRadius: 2 }}
          />
        </Grid>
      </Grid>
      <Skeleton variant="rectangular" height={256} sx={{ borderRadius: 2 }} />
    </Stack>
  );
}
