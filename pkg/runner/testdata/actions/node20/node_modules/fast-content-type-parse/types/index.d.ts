interface ContentType {
  type: string;
  parameters: Record<string, string>;
}

interface FastContentTypeParse {
  parse: (header: string) => ContentType;
  safeParse: (header: string) => ContentType;
  defaultContentType: ContentType;
}

declare namespace fastContentTypeParse {

  export function parse (header: string): ContentType
  export function safeParse (header: string): ContentType
  export const defaultContentType: ContentType

  const fastContentTypeParse: FastContentTypeParse
  export { fastContentTypeParse as default }
}

export = fastContentTypeParse
