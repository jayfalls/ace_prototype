import { Injectable } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from 'rxjs/internal/Observable';
import { environmentURLs } from "../environment";


export type HttpListResponseFailure = { status: string, message: string };


const endpoints = {
    getSettings: `${environmentURLs.controller}/settings`,
};


@Injectable({
  providedIn: "root"
})
export class SettingsService {
  constructor(private http: HttpClient) { }

  getSettings(): Observable<HttpListResponseFailure | any> {
      return this.http.get<HttpListResponseFailure | any>(endpoints.getSettings);
  }
}
