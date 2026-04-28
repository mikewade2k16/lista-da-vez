export function buildNickname(displayName, maxLength = 18) {
  const normalizedName = String(displayName || "").trim();
  const parts = normalizedName.split(/\s+/).filter(Boolean);

  if (!parts.length) {
    return "-";
  }

  const first = parts[0];
  const second = parts.length > 1 ? parts[1] : "";
  const nickname = second ? `${first} ${second.charAt(0).toUpperCase()}.` : first;

  return nickname.length > maxLength ? `${first.slice(0, Math.max(1, maxLength - 2))}...` : nickname;
}