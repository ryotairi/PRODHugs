// Validation rules matching the backend constraints exactly.

const USERNAME_MIN = 3
const USERNAME_MAX = 32
const USERNAME_PATTERN = /^[a-zA-Z][a-zA-Z0-9_]*$/

const PASSWORD_MIN = 8
const PASSWORD_MAX = 128
const HAS_LETTER = /[a-zA-Z]/
const HAS_DIGIT = /[0-9]/
const HAS_SPECIAL = /[^a-zA-Z0-9\s]/

export interface FieldError {
  field: string
  message: string
}

export function validateUsername(value: string): string | null {
  const v = value.trim()
  if (v.length === 0) return 'Введите имя пользователя'
  if (v.length < USERNAME_MIN) return `Минимум ${USERNAME_MIN} символа`
  if (v.length > USERNAME_MAX) return `Максимум ${USERNAME_MAX} символов`
  if (!USERNAME_PATTERN.test(v)) {
    return 'Только латинские буквы, цифры и _ (начинается с буквы)'
  }
  return null
}

export function validatePassword(value: string): string | null {
  if (value.length === 0) return 'Введите пароль'
  if (value.length < PASSWORD_MIN) return `Минимум ${PASSWORD_MIN} символов`
  if (value.length > PASSWORD_MAX) return `Максимум ${PASSWORD_MAX} символов`
  if (!HAS_LETTER.test(value)) return 'Пароль должен содержать хотя бы одну латинскую букву'
  if (!HAS_DIGIT.test(value)) return 'Пароль должен содержать хотя бы одну цифру'
  if (!HAS_SPECIAL.test(value)) return 'Пароль должен содержать хотя бы один спецсимвол (!@#$%...)'
  return null
}

export function validateLoginForm(username: string, password: string): FieldError[] {
  const errors: FieldError[] = []
  if (username.trim().length === 0)
    errors.push({ field: 'username', message: 'Введите имя пользователя' })
  if (password.length === 0) errors.push({ field: 'password', message: 'Введите пароль' })
  return errors
}

export function validateRegisterForm(
  username: string,
  password: string,
  passwordConfirm: string,
): FieldError[] {
  const errors: FieldError[] = []

  const usernameErr = validateUsername(username)
  if (usernameErr) errors.push({ field: 'username', message: usernameErr })

  const passwordErr = validatePassword(password)
  if (passwordErr) errors.push({ field: 'password', message: passwordErr })

  if (password.length > 0 && passwordConfirm.length === 0) {
    errors.push({ field: 'passwordConfirm', message: 'Подтвердите пароль' })
  } else if (password !== passwordConfirm) {
    errors.push({ field: 'passwordConfirm', message: 'Пароли не совпадают' })
  }

  return errors
}

export function validateChangePasswordForm(
  oldPassword: string,
  newPassword: string,
  newPasswordConfirm: string,
): FieldError[] {
  const errors: FieldError[] = []

  if (oldPassword.length === 0)
    errors.push({ field: 'oldPassword', message: 'Введите текущий пароль' })

  const passwordErr = validatePassword(newPassword)
  if (passwordErr) errors.push({ field: 'newPassword', message: passwordErr })

  if (newPassword.length > 0 && newPasswordConfirm.length === 0) {
    errors.push({ field: 'newPasswordConfirm', message: 'Подтвердите новый пароль' })
  } else if (newPassword !== newPasswordConfirm) {
    errors.push({ field: 'newPasswordConfirm', message: 'Пароли не совпадают' })
  }

  return errors
}

// -------------------------------------------------------------------
// Backend error parsing
// -------------------------------------------------------------------

// Handler errors: { code: string, message: string }
// OpenAPI validation errors: { type: string, title: string, status: number, detail: string }

interface BackendHandlerError {
  code: string
  message: string
}

interface BackendValidationError {
  type: string
  title: string
  status: number
  detail: string
}

type BackendError = BackendHandlerError | BackendValidationError

function isHandlerError(data: any): data is BackendHandlerError {
  return typeof data?.code === 'string' && typeof data?.message === 'string'
}

function isValidationError(data: any): data is BackendValidationError {
  return data?.type === 'validation_error' && typeof data?.detail === 'string'
}

// Known handler error codes mapped to { field, russian message }
const HANDLER_ERROR_MAP: Record<string, { field: string | null; message: string }> = {
  WEAK_PASSWORD: { field: 'password', message: 'Пароль не соответствует требованиям безопасности' },
  USER_ALREADY_EXISTS: { field: 'username', message: 'Пользователь с таким именем уже существует' },
  INVALID_CREDENTIALS: { field: null, message: 'Неверное имя пользователя или пароль' },
  WRONG_PASSWORD: { field: 'oldPassword', message: 'Неверный текущий пароль' },
  USER_BANNED: {
    field: null,
    message: 'Ваш аккаунт заблокирован. Обратитесь к администратору.',
  },
  CANNOT_BAN_ADMIN: { field: null, message: 'Невозможно заблокировать администратора' },
  TELEGRAM_LOGIN_FAILED: { field: null, message: 'Не удалось войти через Telegram' },
  INVALID_MATRIX_ID: { field: 'matrix_id', message: 'Неверный формат MXID (пример: @user:matrix.org)' },
  MATRIX_ID_TAKEN: { field: 'matrix_id', message: 'Этот Matrix аккаунт уже привязан к другому пользователю' },
  MATRIX_LOGIN_FAILED: { field: null, message: 'Не удалось войти через Matrix' },
}

export interface ParsedBackendError {
  fieldErrors: FieldError[]
  generalError: string | null
}

/**
 * Parse an axios error response from the backend into field-level
 * and general errors that the UI can display inline.
 */
export function parseBackendError(e: any): ParsedBackendError {
  const data: BackendError | undefined = e?.response?.data
  const result: ParsedBackendError = { fieldErrors: [], generalError: null }

  if (!data) {
    result.generalError = 'Ошибка соединения с сервером'
    return result
  }

  // Handler-level error with a known code
  if (isHandlerError(data)) {
    const mapped = HANDLER_ERROR_MAP[data.code]
    if (mapped) {
      if (mapped.field) {
        result.fieldErrors.push({ field: mapped.field, message: mapped.message })
      } else {
        result.generalError = mapped.message
      }
    } else {
      // Unknown code — use the backend message as-is
      result.generalError = data.message
    }
    return result
  }

  // OpenAPI validation error — try to extract the field name from detail
  if (isValidationError(data)) {
    const detail = String(data.detail)

    // detail is typically: "request body has an error: doesn't match schema: Error at \"/password\": ..."
    const fieldMatch = detail.match(/Error at "\/(\w+)"/)
    const field = fieldMatch?.[1] ?? null

    // Extract the useful part after the last colon
    let msg = detail
    const schemaIdx = detail.indexOf("doesn't match schema:")
    if (schemaIdx !== -1) {
      msg = detail.slice(schemaIdx + "doesn't match schema:".length).trim()
    }

    if (field) {
      result.fieldErrors.push({ field, message: humanizeValidationDetail(field, msg) })
    } else {
      result.generalError = 'Ошибка валидации данных'
    }
    return result
  }

  // Fallback
  result.generalError = (data as any)?.message ?? 'Неизвестная ошибка'
  return result
}

/**
 * Turn raw OpenAPI validation messages into human-readable Russian text.
 */
function humanizeValidationDetail(field: string, detail: string): string {
  const d = detail.toLowerCase()

  if (field === 'username') {
    if (d.includes('minimum string length')) return `Минимум ${USERNAME_MIN} символа`
    if (d.includes('maximum string length')) return `Максимум ${USERNAME_MAX} символов`
    if (d.includes('pattern')) return 'Только латинские буквы, цифры и _ (начинается с буквы)'
    return 'Некорректное имя пользователя'
  }

  if (field === 'password' || field === 'new_password') {
    if (d.includes('minimum string length')) return `Минимум ${PASSWORD_MIN} символов`
    if (d.includes('maximum string length')) return `Максимум ${PASSWORD_MAX} символов`
    return 'Некорректный пароль'
  }

  return 'Некорректное значение'
}
