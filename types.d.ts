declare module 'better-sqlite3' {
  interface Database {
    pragma(statement: string): any;
    prepare(statement: string): any;
    exec(statement: string): any;
    close(): void;
  }
  
  namespace Database {
    interface Database {
      pragma(statement: string): any;
      prepare(statement: string): any;
      exec(statement: string): any;
      close(): void;
    }
  }
  
  function Database(filename: string): Database;
  export = Database;
}

declare module 'uuid' {
  export function v4(): string;
}