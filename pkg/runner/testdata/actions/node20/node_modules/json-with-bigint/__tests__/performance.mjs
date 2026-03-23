// ------ Performance tests ------

"use strict";

import { performance } from "perf_hooks";
import { imitateJSONParseWithoutContext } from "./helpers.cjs";
import { JSONParse, JSONStringify } from "../json-with-bigint.js";

import { promises as fs } from "fs";
import { get } from "https";

// JSON is located in a separate GitHub repo here:
// https://github.com/Ivan-Korolenko/json-with-bigint.performance.json/blob/main/performance.json
const JSON_URL =
  "https://raw.githubusercontent.com/Ivan-Korolenko/json-with-bigint.performance.json/refs/heads/main/performance.json";
const JSON_LOCAL_FILEPATH = "__tests__/performance.json";

async function fetchJSON(url, maxRetries = 3, delay = 1000) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const response = await new Promise((resolve, reject) => {
        get(url, (res) => {
          if (res.statusCode >= 500 && res.statusCode < 600) {
            reject(new Error(`Server error ${res.statusCode}: Retrying...`));
          } else if (res.statusCode !== 200) {
            reject(
              new Error(
                `Request failed with status ${res.statusCode} ${res.statusMessage}`,
              ),
            );
          }

          let data = "";
          res.on("data", (chunk) => {
            data += chunk;
          });
          res.on("end", () => resolve(data));
        }).on("error", reject);
      });

      return JSON.parse(response);
    } catch (error) {
      if (attempt < maxRetries) {
        console.warn(`Attempt ${attempt} failed: ${error.message}`);
        await new Promise((res) =>
          setTimeout(res, delay * Math.pow(2, attempt - 1)),
        ); // Exponential backoff
      } else {
        console.error("Max retries reached. Fetch failed:", error);
        throw error;
      }
    }
  }
}

async function saveJSONToFile(filePath, data) {
  try {
    const jsonString = JSON.stringify(data, null, 2);
    const tempPath = `${filePath}.tmp`;

    await fs.writeFile(tempPath, jsonString, "utf8");
    await fs.rename(tempPath, filePath); // Atomic write
    console.log(`✅ JSON data saved to ${filePath}`);
  } catch (error) {
    console.error("Error saving JSON to file:", error);
    throw error;
  }
}

async function fetchAndSaveJSON(url, filePath) {
  try {
    const jsonData = await fetchJSON(url);

    await saveJSONToFile(filePath, jsonData);
  } catch (error) {
    console.error("❌ Operation failed:", error);
  }
}

// If the file was downloaded earlier, use it. Otherwise, download
// After n attempts, give up and throw an error
async function readPerformanceJSON(
  filePath,
  encoding,
  maxAttempts = 3,
  attempt = 0,
) {
  if (attempt === maxAttempts)
    throw new Error(
      `Reading performance JSON failed after ${attempt} attempts. Check download URL, file availability on that URL, and local filepath`,
    );

  try {
    const data = await fs.readFile(filePath, encoding);

    return data;
  } catch (error) {
    if (error.code === "ENOENT") {
      console.log(
        `File not found. Downloading... (Attempt ${attempt + 1}/${maxAttempts})`,
      );
      await fetchAndSaveJSON(JSON_URL, filePath);

      return await readPerformanceJSON(
        filePath,
        encoding,
        maxAttempts,
        attempt + 1,
      );
    } else {
      console.error("Error reading file:", error);
      throw error; // Re-throw to avoid silent failures
    }
  }
}

const measureExecTime = (fn) => {
  const startTime = performance.now();

  fn();

  const endTime = performance.now();

  console.log("Time: ", endTime - startTime);
};

const runTests = (data) => {
  console.log("___________\nPerformance test. One-way");
  measureExecTime(() => JSONParse(data));

  console.log("___________\nPerformance test. Round-trip");
  measureExecTime(() => JSONStringify(JSONParse(data)));
};

async function main() {
  const data = await readPerformanceJSON(JSON_LOCAL_FILEPATH, "utf8");

  console.log("------ V2 performance tests ------");
  runTests(data);

  console.log("\n------ V1 (without context.source) performance tests ------");
  JSON.parse = imitateJSONParseWithoutContext;
  runTests(data);
}

main();
