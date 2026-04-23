const CONSULTANT_COLORS = ["#168aad", "#7a6ff0", "#d17a96", "#e09f3e", "#355070", "#d90429", "#23a26d", "#4361ee"];

export function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

export function sampleItems(items, count) {
  const pool = [...items];
  const picked = [];

  while (pool.length > 0 && picked.length < count) {
    const index = randomInt(0, pool.length - 1);
    picked.push(pool[index]);
    pool.splice(index, 1);
  }

  return picked;
}

export function slugifyLabel(label) {
  return String(label || "")
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

export function createOptionId(prefix, label, existingItems = []) {
  const items = Array.isArray(existingItems) ? existingItems : [];
  const base = `${prefix}-${slugifyLabel(label) || "item"}`;
  let candidate = base;
  let cursor = 2;

  while (items.some((item) => item.id === candidate)) {
    candidate = `${base}-${cursor}`;
    cursor += 1;
  }

  return candidate;
}

export function findOptionByLabel(options = [], label) {
  const normalizedLabel = String(label || "").trim().toLowerCase();

  if (!normalizedLabel) {
    return null;
  }

  return options.find((item) => String(item?.label || "").trim().toLowerCase() === normalizedLabel) || null;
}

export function appendUniqueOption(options, prefix, label) {
  const normalizedLabel = String(label || "").trim();

  if (!normalizedLabel) {
    return {
      item: null,
      items: Array.isArray(options) ? options : []
    };
  }

  const currentItems = Array.isArray(options) ? options : [];
  const existing = findOptionByLabel(currentItems, normalizedLabel);

  if (existing) {
    return {
      item: existing,
      items: currentItems
    };
  }

  const nextItem = {
    id: createOptionId(prefix, normalizedLabel, currentItems),
    label: normalizedLabel
  };

  return {
    item: nextItem,
    items: [...currentItems, nextItem]
  };
}

export function buildConsultantInitials(name) {
  const parts = String(name || "")
    .trim()
    .split(/\s+/)
    .filter(Boolean);

  if (!parts.length) {
    return "CO";
  }

  const first = parts[0].charAt(0);
  const second = parts.length > 1 ? parts[1].charAt(0) : parts[0].charAt(1) || "X";

  return `${first}${second}`.toUpperCase();
}

export function buildConsultantColor(existingRoster = []) {
  const usedColors = new Set((existingRoster || []).map((item) => item.color));
  const availableColor = CONSULTANT_COLORS.find((color) => !usedColors.has(color));

  return availableColor || CONSULTANT_COLORS[Math.floor(Math.random() * CONSULTANT_COLORS.length)];
}

export function getActiveProfile(state) {
  return (state.profiles || []).find((profile) => profile.id === state.activeProfileId) || state.profiles?.[0] || null;
}

export function getCurrentRole(state) {
  return getActiveProfile(state)?.role || "consultant";
}
