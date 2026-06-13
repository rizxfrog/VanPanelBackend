type ZodSchema = ReturnType<typeof ZodString> | ReturnType<typeof ZodObject>;

function safeParser<T>(parse: (value: unknown) => T): {
  parse: (value: unknown) => T;
  safeParse: (value: unknown) => { success: true; data: T } | { success: false; error: Error };
} {
  return {
    parse: (value) => {
      try {
        return parse(value);
      } catch (e) {
        throw e instanceof Error ? e : new Error(String(e));
      }
    },
    safeParse: (value) => {
      try {
        const data = parse(value);
        return { success: true as const, data };
      } catch (error) {
        return { success: false as const, error: error instanceof Error ? error : new Error(String(error)) };
      }
    },
  };
}

function ZodString(props?: { max?: number; optional?: boolean }) {
  const maxLen = props?.max;
  const isOptional = props?.optional ?? false;

  const parse = (value: unknown): string | undefined => {
    if (value === undefined || value === null) {
      if (isOptional) return undefined;
      throw new Error("Required");
    }
    if (typeof value !== "string") throw new Error("Expected string");
    if (maxLen !== undefined && value.length > maxLen) throw new Error(`String too long (max ${maxLen})`);
    return value;
  };

  const result = {
    ...safeParser(parse),
    max: (n: number) => ZodString({ max: n, optional: isOptional }),
    optional: () => ZodString({ max: maxLen, optional: true }),
    _def: { typeName: "ZodString" as const },
  };

  return result;
}

function ZodObject<T extends Record<string, ReturnType<typeof ZodString>>>(shape: T) {
  const optional = false;
  const parse = (value: unknown) => {
    if (value === undefined || value === null) {
      if (optional) return undefined as unknown as { [K in keyof T]: ReturnType<T[K]["parse"]> };
      throw new Error("Required");
    }
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Expected object");
    const record = value as Record<string, unknown>;
    const result: Record<string, unknown> = {};
    for (const [key, schema] of Object.entries(shape)) {
      result[key] = (schema as { parse: (v: unknown) => unknown }).parse(record[key]);
    }
    return result as { [K in keyof T]: ReturnType<T[K]["parse"]> };
  };

  const result = {
    ...safeParser(parse),
    optional: () => ZodObjectOptional(shape),
    _def: { typeName: "ZodObject" as const, shape: () => shape },
  };

  return result;
}

function ZodObjectOptional<T extends Record<string, ReturnType<typeof ZodString>>>(shape: T) {
  const parse = (value: unknown) => {
    if (value === undefined || value === null) return undefined;
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Expected object");
    const record = value as Record<string, unknown>;
    const result: Record<string, unknown> = {};
    for (const [key, schema] of Object.entries(shape)) {
      result[key] = (schema as { parse: (v: unknown) => unknown }).parse(record[key]);
    }
    return result as { [K in keyof T]: ReturnType<T[K]["parse"]> };
  };

  return {
    ...safeParser(parse),
    optional: () => ZodObjectOptional(shape),
    _def: { typeName: "ZodObject" as const, shape: () => shape },
  };
}

export const z = {
  string: () => ZodString(),
  object: ZodObject,
};
