/** Java String.hashCode() uses UTF-16 code units, including both surrogate halves. */
export function javaStringHash(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index++) {
    hash = (Math.imul(hash, 31) + value.charCodeAt(index)) | 0;
  }
  return hash;
}

/**
 * CLDC 1.1 guarantees default case conversion for ISO Latin-1; other Unicode
 * blocks depend on the device. Do not silently substitute modern JavaScript's
 * full Unicode mappings (for example, U+0130 expands to two code points).
 *
 * Non-Latin-1 characters that need no lowercase conversion pass unchanged.
 * Other mappings require an explicitly verified target-device policy.
 * Source: https://docs.oracle.com/javame/config/cldc/ref-impl/cldc1.1/jsr139/java/lang/Character.html
 */
export function lowercasePortablePassword(password: string): string {
  let result = '';
  for (const character of password) {
    const code = character.codePointAt(0)!;
    if ((code >= 0x41 && code <= 0x5a) ||
        (code >= 0xc0 && code <= 0xd6) || (code >= 0xd8 && code <= 0xde)) {
      result += String.fromCodePoint(code + 32);
    } else {
      if (code > 0xff && character.toLowerCase() !== character) {
        throw new RangeError('Password needs an unverified non-Latin-1 case mapping');
      }
      result += character;
    }
  }
  return result;
}

export function loginChallengeHash(
  challenge: string,
  password: string,
  lowercase: (password: string) => string = lowercasePortablePassword,
): number {
  return javaStringHash(challenge + lowercase(password));
}
