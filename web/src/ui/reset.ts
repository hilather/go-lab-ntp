export const RESET_PHRASE = "RESET";

export function canSubmitReset(phrase: string, confirmed: boolean, allowed: boolean): boolean {
  return allowed && confirmed && phrase.trim() === RESET_PHRASE;
}
