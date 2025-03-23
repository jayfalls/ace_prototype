// DEPENDENCIES
//// Built-In
import { Injectable } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from 'rxjs/internal/Observable';
//// Local
import { APIRoutes } from "../constants";
import { environmentURLs } from "../environment";
import { ISettings } from "../models/settings.models";


export type HttpListResponseFailure = { status: string, message: string };


const endpoints = {
    getSettings: `${environmentURLs.controller}${APIRoutes.ROOT}settings`,
    editSettings: `${environmentURLs.controller}${APIRoutes.ROOT}settings`,
    resetSettings: `${environmentURLs.controller}${APIRoutes.ROOT}settings`
};


@Injectable({
  providedIn: "root"
})
export class SettingsService {
  constructor(private http: HttpClient) { }

  getSettings(): Observable<HttpListResponseFailure | any> {
    return this.http.get<HttpListResponseFailure | any>(endpoints.getSettings);
  }

  editSettings(settings: ISettings): Observable<HttpListResponseFailure | any> {
    return this.http.post<HttpListResponseFailure | any>(endpoints.editSettings, settings);
  }

  resetSettings(): Observable<HttpListResponseFailure | any> {
    return this.http.delete<HttpListResponseFailure | any>(endpoints.resetSettings);
  }
}
