import * as age from 'age-encryption'

const AGE_HEADER = new TextEncoder().encode('age-encryption.org/v1\n')

/** Returns true if the bytes look like an age binary-format ciphertext. */
export function isAgeEncrypted(data: Uint8Array): boolean {
  if (data.length < AGE_HEADER.length) return false
  for (let i = 0; i < AGE_HEADER.length; i++) {
    if (data[i] !== AGE_HEADER[i]) return false
  }
  return true
}

/** Encrypts a UTF-8 string with the given passphrase and returns the age ciphertext. */
export async function encryptWithPassphrase(plaintext: string, passphrase: string): Promise<Uint8Array> {
  const e = new age.Encrypter()
  e.setPassphrase(passphrase)
  return e.encrypt(new TextEncoder().encode(plaintext))
}

/** Decrypts age ciphertext with the given passphrase and returns the plaintext string. */
export async function decryptWithPassphrase(ciphertext: Uint8Array, passphrase: string): Promise<string> {
  const d = new age.Decrypter()
  d.addPassphrase(passphrase)
  const plaintext = await d.decrypt(ciphertext)
  return new TextDecoder().decode(plaintext)
}
