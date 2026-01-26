const DEFAULT_NAMESPACE = "default";
const STORAGE_KEY = "surge-selected-namespace";

export const getStoredNamespace = () => {
  try {
    return localStorage.getItem(STORAGE_KEY) || DEFAULT_NAMESPACE;
  } catch {
    return DEFAULT_NAMESPACE;
  }
};

export const saveNamespace = (namespace) => {
  try {
    localStorage.setItem(STORAGE_KEY, namespace);
  } catch (err) {
    console.warn("Failed to save namespace selection:", err);
  }
};

export const sortNamespaces = (namespaces) => {
  return [...namespaces].sort((a, b) => {
    if (a === DEFAULT_NAMESPACE) return -1;
    if (b === DEFAULT_NAMESPACE) return 1;
    return a.localeCompare(b);
  });
};

export const selectInitialNamespace = (sortedNamespaces, onNamespaceChange) => {
  const persistedNs = getStoredNamespace();
  
  if (sortedNamespaces.includes(persistedNs)) {
    onNamespaceChange(persistedNs);
    return persistedNs;
  }
  
  if (sortedNamespaces.includes(DEFAULT_NAMESPACE)) {
    onNamespaceChange(DEFAULT_NAMESPACE);
    saveNamespace(DEFAULT_NAMESPACE);
    return DEFAULT_NAMESPACE;
  }
  
  const firstNamespace = sortedNamespaces[0];
  onNamespaceChange(firstNamespace);
  saveNamespace(firstNamespace);
  return firstNamespace;
};

export const selectFallbackNamespace = (sortedNamespaces, currentNamespace, onNamespaceChange) => {
  if (sortedNamespaces.includes(currentNamespace)) {
    return currentNamespace;
  }
  
  const fallback = sortedNamespaces.includes(DEFAULT_NAMESPACE) 
    ? DEFAULT_NAMESPACE 
    : sortedNamespaces[0];
  
  onNamespaceChange(fallback);
  saveNamespace(fallback);
  return fallback;
};
