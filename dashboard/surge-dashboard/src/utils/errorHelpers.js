const BACKEND_ERROR_MESSAGE = "Backend server not available";

export const shouldLogError = (error) => {
  return !error?.message?.includes(BACKEND_ERROR_MESSAGE);
};

export const handleApiError = (error, context = "") => {
  if (shouldLogError(error)) {
    console.error(`${context ? context + ": " : ""}${error.message || error}`);
  }
};
